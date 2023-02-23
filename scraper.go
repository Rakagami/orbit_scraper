package main

import (
	"database/sql"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
}

// Info about a source file without containing the actual file
type SourceFileInfo struct {
    Filename        string
    LastAccessed    time.Time
    FileHash        string
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
                log.Println("Found: ", e.Text)
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

func DownloadTles(dir string, items []ScrapedItem) {
    log.Println("Scraped Data:")
    for _, element := range items {
        log.Println("Constellation:", element.Name)
        log.Println("URL:", element.URL)
        resp, err := http.Get(element.URL)
        if err != nil {
            log.Panic(err)
        }
        body, _ := io.ReadAll(resp.Body)
        filePath := filepath.Join(dir, element.Name + ".txt")
        if err := os.WriteFile(filePath, body, 0666); err != nil {
            log.Panic(err)
        }
        resp.Body.Close()

        element.TLEFile = filePath
    }
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


func ParseTle(line0 string, line1 string, line2 string) (OrbitData, SourceFileInfo) {
    satcatNum, _        := strconv.ParseInt(line1[2:7], 10, 32)
    epoch               := ParseEpoch(line1[18:32])
    inclinationDeg, _   := strconv.ParseFloat(line2[8:16], 32)
    raanDeg, _          := strconv.ParseFloat(line2[17:25], 32)
    meanMotion, _       := strconv.ParseFloat(line2[52:63], 32) // in rev/day

    // Some astrophysics magic calculations
    MU      := 398600.4418 // unit: (km)^3 / s^2
    EARTH_R := 6371.0 // unit: km

    periodS := 86400.0 / meanMotion
    semimajoraxis       := math.Pow((math.Sqrt(MU) * periodS) / (2.0*math.Pi), 2.0/3.0)

    altitudeKm          := semimajoraxis - EARTH_R

    orbitData   := OrbitData{
        TleLine0: line0,
        TleLine1: line1,
        TleLine2: line2,
        SatcatNum: int(satcatNum),
        Epoch: epoch,
        Inclination: float32(inclinationDeg),
        RAAN: float32(raanDeg),
        Altitude: float32(altitudeKm),
        Period: float32(meanMotion),
    }
    srcFileInfo := SourceFileInfo{}

    return orbitData, srcFileInfo
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
        Name varchar(30)
    );
	create table Satellites (
        ID integer primary key,
        ConstellationsID int NOT NULL,
        LaunchDate datetime,
        FOREIGN KEY (ConstellationsID) REFERENCES Constellations(ID)
    );
	create table SatelliteOrbits (
        ID integer primary key,
        TLE text,
        SatellitesID integer NOT NULL,
        ORBIT_Epoch datetime,
        ORBIT_InclinationDeg float,
        ORBIT_RAANDeg float,
        ORBIT_AltitudeKm float,
        ORBIT_PeriodS float,
        FILE_Hash string,
        FILE_URL string,
        FILE_LastAccessed datetime,
        FOREIGN KEY (SatellitesID) REFERENCES Satellites(ID)
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
    tx, err := db.Begin()
	constStmt, err      := tx.Prepare("insert into Constellations(name) values(?)")
    defer constStmt.Close()
	//satStmt, err        := tx.Prepare("insert into foo(id, name) values(?, ?)")
    //defer satStmt.Close()
	//satOrbitStmt, err   := tx.Prepare("insert into foo(id, name) values(?, ?)")
	//defer satOrbitStmt.Close()
    _, err = constStmt.Exec(item.Name)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func AddDB(sqlitePath string, items []ScrapedItem) {
    sqlite3.Version()

    db, err := sql.Open("sqlite3", sqlitePath)
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

    for _, item := range items {
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
    DownloadTles(dir, items)
    
    // -----------------------------------------------------------
    // sqlite3
    // -----------------------------------------------------------
    sqlitePath := "./db.sqlite"
    if _, err := os.Stat(sqlitePath); err != nil {
        InitDB(sqlitePath)
    }
    AddDB(sqlitePath, items)
}
