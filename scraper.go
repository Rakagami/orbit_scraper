package main

import (
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/mattn/go-sqlite3"
)

// Simplified Orbit data for LEO understanding
//  - We assume circular orbit
type OrbitData struct {
    TleLine0    string
    TleLine1    string
    TleLine2    string
    SatcatNum   int
    Epoch       time.Time
    Inclination float32 // in degrees
    RAAN        float32 // in degrees
    Altitude    float32 // We use semimajor axis - earth radi for this
    Period      float32 // a period of a revolution in seconds
    ElementSetNumber int
}

type ScrapedItem struct {
    Name    string
    URL     string
    TLEFile string
}

func ScrapeCelestrak(items *[]ScrapedItem) int {
    site_url := "https://celestrak.org/NORAD/elements/supplemental/"

    c := colly.NewCollector(
        colly.AllowedDomains("celestrak.org"),
    )

    c.OnHTML("table.center.outline.striped > tbody td.center", func(e *colly.HTMLElement) {
        e.ForEachWithBreak("a[href]", func(i int, e *colly.HTMLElement) bool {
            if i == 0 {
                //log.Println("Found: ", e.Text)
                //tle_url, _ := url.JoinPath(site_url, e.Attr("href"))
                *items = append(*items, ScrapedItem{
                    Name: e.Text,
                    URL: site_url + e.Attr("href"),
                })
                //log.Println(e.Attr("href"))
                //log.Println(site_url + e.Attr("href"))
                return false
            }
            return true
        })
    })

    log.Printf("Visiting %q...\n", site_url)
    c.Visit(site_url)

    return len(*items)
}

func DownloadTles(dir string, item *ScrapedItem) {
    //log.Println("Constellation:", item.Name)
    //log.Println("URL:", item.URL)
    resp, err := http.Get(item.URL)
    if err != nil {
        log.Panic(err)
    }
    body, _ := io.ReadAll(resp.Body)
    filePath := filepath.Join(dir, item.Name + ".txt")
    if err := os.WriteFile(filePath, body, 0666); err != nil {
        log.Panic(err)
    }
    resp.Body.Close()
    
    item.TLEFile = filePath
}

func ParseEpoch(epochStr string) time.Time {
    // the epochStr is the 19-32 idx characters of the first line of a TLE
    year, _  := strconv.ParseInt(epochStr[:2], 10, 32)
    if year > 56 { // oh god, why is TLE like this...
        year += 1900
    } else {
        year += 2000
    }
    epochDay, _ := strconv.ParseFloat(epochStr[2:], 32)
    epochNS := epochDay * 24 * 3600 * 1000000000 // epoch as nanoseconds

    date := time.Date(int(year), 1, 0, 0, 0, 0, int(epochNS), time.UTC)

    return date
}


func ParseTle(line0 string, line1 string, line2 string) OrbitData {
    satcatNum, _        := strconv.ParseInt(strings.TrimSpace(line1[2:7]), 10, 64)
    epoch               := ParseEpoch(strings.TrimSpace(line1[18:32]))
    inclinationDeg, _   := strconv.ParseFloat(strings.TrimSpace(line2[8:16]), 64)
    raanDeg, _          := strconv.ParseFloat(strings.TrimSpace(line2[17:25]), 64)
    meanMotion, _       := strconv.ParseFloat(strings.TrimSpace(line2[52:63]), 64) // in rev/day
    elementSetNumber, _ := strconv.ParseInt(strings.TrimSpace(line1[64:68]), 10, 64)

    // Some astrophysics magic calculations
    MU      := 398600.4418 // unit: (km)^3 / s^2
    EARTH_R := 6371.0 // unit: km

    periodS := 86400.0 / meanMotion
    semimajoraxis       := math.Pow((math.Sqrt(MU) * periodS) / (2.0*math.Pi), 2.0/3.0)

    altitudeKm          := semimajoraxis - EARTH_R

    orbitData   := OrbitData{
        TleLine0: strings.TrimSpace(line0),
        TleLine1: line1,
        TleLine2: line2,
        SatcatNum: int(satcatNum),
        Epoch: epoch,
        Inclination: float32(inclinationDeg),
        RAAN: float32(raanDeg),
        Altitude: float32(altitudeKm),
        Period: float32(meanMotion),
        ElementSetNumber: int(elementSetNumber),
    }

    return orbitData
}

// We define the following table structure we want to parse:
//
// Constellation
// ID: string, Name: string
//
// Satellites
// ID: string, LaunchDate: date, ConstellationID: string
//
// SatelliteOrbits
// ID: string, SatelliteID: string, OrbitData: orbitdata, TLE: string, SourceFileInfo: sourcefileinfo
//
func InitDB(sqlitePath string) {
    log.Println("Initializing a new DB...")
    os.Remove(sqlitePath)
    db, err := sql.Open("sqlite3", sqlitePath)
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

	sqlStmt := `
	create table Constellations (
        ID integer primary key,
        Name varchar(30) unique
    );
	create table Satellites (
        SATCATID integer primary key,
        ConstellationsID int,
        LaunchDate datetime,
        FOREIGN KEY (ConstellationsID) REFERENCES Constellations(ID)
    );
	create table SatelliteOrbits (
        ID integer primary key,
        SATCATID integer NOT NULL,
        ORBIT_TLE0 text,
        ORBIT_TLE1 text,
        ORBIT_TLE2 text,
        ORBIT_Epoch datetime,
        ORBIT_InclinationDeg float,
        ORBIT_RAANDeg float,
        ORBIT_AltitudeKm float,
        ORBIT_PeriodS float,
        ORBIT_ElementSetNumber int,
        FILE_Hash string,
        FILE_URL string,
        FILE_LastAccessed datetime,
        FOREIGN KEY (SATCATID) REFERENCES Satellites(SATCATID)
    );
	`
    _, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
    log.Println("Created DB in ", sqlitePath)
}

func ParseNInsert(item ScrapedItem, db *sql.DB) {
    sqlite3.Version()

    // ----------------------------------
    // Constellation related transactions
    // ----------------------------------
    tx, err := db.Begin()
	constStmt, err      := tx.Prepare(`insert or ignore into Constellations(name) values(?)`)
    defer constStmt.Close()

    // Get Constellation ID
    rows, err := db.Query(fmt.Sprintf(
        "select * from Constellations where Name = %q", item.Name))
	if err != nil {
		log.Fatal(err)
	}
    var constellationID int64
    if rows.Next() {
		var name string
		err = rows.Scan(&constellationID, &name)
        log.Printf("Constellation %q exists - Constellation Id: %d", item.Name, constellationID)
		if err != nil {
			log.Panic(err)
		}
	} else {
        log.Println("Creating new Constellation entry...")
        res, err := constStmt.Exec(item.Name)
        if err != nil {
            log.Panic(err)
        } else {
            constellationID, _ = res.LastInsertId()
            log.Printf("Entered Constellation %q - Constellation Id: %d", item.Name, constellationID)
        }
    }
	err = tx.Commit()
	if err != nil {
		log.Panic(err)
	}

    // ----------------------------------
    // Parse TLE and add Satellite Orbits
    // ----------------------------------
    tx, err = db.Begin()
	satStmt, err        := tx.Prepare(`insert or ignore into Satellites(
        SATCATID,
        ConstellationsID
    ) values(?, ?)`)
    defer satStmt.Close()
	satOrbitStmt, err   := tx.Prepare(`insert into SatelliteOrbits(
        SATCATID,
        ORBIT_TLE0,
        ORBIT_TLE1,
        ORBIT_TLE2,
        ORBIT_Epoch,
        ORBIT_InclinationDeg,
        ORBIT_RAANDeg,
        ORBIT_AltitudeKm,
        ORBIT_PeriodS,
        ORBIT_ElementSetNumber,
        FILE_Hash,
        FILE_URL,
        FILE_LastAccessed
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	defer satOrbitStmt.Close()

    //log.Println("Parse TLE and add Satellite Orbits")
    content, err := os.ReadFile(item.TLEFile)
    if err != nil {
        log.Fatal(err)
    }
    h := sha256.New()
    h.Write([]byte(content))
    splitLines := strings.Split(string(content), "\n")
    var nLines int = len(splitLines) / 3
    for i := 0; i < nLines; i++ {
        idx := i * 3
        orbitData := ParseTle(splitLines[idx], splitLines[idx+1], splitLines[idx+2])
        //log.Println(orbitData.Epoch.Format("2006-01-02 15:04:05"))
        //log.Println(srcFileInfo.LastAccessed.Format("2006-01-02 15:04:05"))
        //log.Printf("New Satellite %d and Database ID %d\n", orbitData.SatcatNum, constellationID)
        _ = constellationID
        _, err = satStmt.Exec(
            orbitData.SatcatNum,
            constellationID,
        )
        if err != nil {
            log.Panic(err)
        }

        _, err = satOrbitStmt.Exec(
            orbitData.SatcatNum,
            orbitData.TleLine0,
            orbitData.TleLine1,
            orbitData.TleLine2,
            orbitData.Epoch.Format("2006-01-02 15:04:05"),
            orbitData.Inclination,
            orbitData.RAAN,
            orbitData.Altitude,
            orbitData.Period,
            orbitData.ElementSetNumber,
            fmt.Sprintf("%x", h.Sum(nil)),
            item.URL,
            time.Now().Format("2006-01-02 15:04:05"),
        )
        if err != nil {
            log.Panic(err)
        }
    }

	err = tx.Commit()
	if err != nil {
		log.Panic(err)
	}
}

func AddDB(sqlitePath string, items *[]ScrapedItem) {
    sqlite3.Version()

    db, err := sql.Open("sqlite3", "file:" + sqlitePath + "?cache=shared")
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

    for _, item := range *items {
	    if err != nil {
	    	log.Fatal(err)
	    }
        ParseNInsert(item, db)
	    if err != nil {
	    	log.Fatal(err)
	    }
    }
}

func main() {
    log.SetOutput(os.Stdout)
    log.Println("Starting Scraper...")

    var dbPath string
    flag.StringVar(&dbPath, "d", "scraped.db", "give db path")
	flag.Parse()

    // -----------------------------------------------------------
    // Scrape constellations
    // -----------------------------------------------------------
    items := make([]ScrapedItem, 0, 5)
    itemcnt := ScrapeCelestrak(&items)
    log.Printf("Finished web scraping. Found %d items\n", itemcnt)

    // -----------------------------------------------------------
    // Create temporary directory where orbit files are downloaded
    // -----------------------------------------------------------
    dir, err := os.MkdirTemp("", "scrapertemp")
    if err != nil {
		log.Panic(err)
	}
    defer os.RemoveAll(dir)
    log.Println("Created temporary dir", dir)
    for i, item := range items {
        DownloadTles(dir, &item)
        items[i] = item
    }
    
    // -----------------------------------------------------------
    // sqlite3
    // -----------------------------------------------------------
    if _, err := os.Stat(dbPath); err != nil {
        InitDB(dbPath)
    }
    AddDB(dbPath, &items)

    log.Println("Finished")
}
