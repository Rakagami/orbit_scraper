package main

//import (
//	"crypto/sha256"
//	"flag"
//	"fmt"
//	"log"
//	"os"
//	"strings"
//	"time"
//
//	"github.com/gocolly/colly"
//	"github.com/mattn/go-sqlite3"
//)

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gocolly/colly"
)

type ScrapedSatellite struct {
    COSPAR          string
    StarlinkName    string
    LaunchDate      string
    Group           string
    Revision        string
}

// This method requires some domain knowledge about Starlink
// v1.0 consists of Group 1
// v1.5 revision contains Group 2, 3, 4, 5
// v2.m so far, only Group 6
func ParseStarlinkRow(e *colly.HTMLElement) *ScrapedSatellite {
    if(e.Index == 0) {
        return nil
    } else if(len(e.ChildText("td.cosid")) < 3) {
        return nil
    }

    sat := &ScrapedSatellite{}
    e.ForEach("td", func(i int, ce *colly.HTMLElement) {
        switch i {
        case 0:
            splitstr := strings.SplitN(ce.Text, " ", 4)
            if(len(splitstr) < 4) {
                sat.StarlinkName = ""
            } else {
                sat.StarlinkName = splitstr[3][1:len(splitstr[3])-1]
            }
            sat.Group = splitstr[2][:2]
            sat.Revision = splitstr[1]
        case 1:
            sat.COSPAR = ce.Text
        case 2:
            sat.LaunchDate = ce.Text
        }
    })
    if sat.Revision == "v1.0" {
        sat.Group = "G1"
    }
    fmt.Println(e.Index, "Parsed Row:", sat)
    return sat
}

func ScrapeStarlink(items *[]ScrapedSatellite) int {
    v0_9_site := "https://space.skyrocket.de/doc_sdat/starlink-v0-9.htm"
    v1_0_site := "https://space.skyrocket.de/doc_sdat/starlink-v1-0.htm"
    v1_5_site := "https://space.skyrocket.de/doc_sdat/starlink-v1-5.htm"
    v2_m_site := "https://space.skyrocket.de/doc_sdat/starlink-v2-mini.htm"
    v2_0_site := "https://space.skyrocket.de/doc_sdat/starlink-v2-0-ss.htm"
    sites := []string{v0_9_site, v1_0_site, v1_5_site, v2_m_site, v2_0_site}

    c := colly.NewCollector(
        colly.AllowedDomains("space.skyrocket.de"),
    )

    c.OnHTML("table#satlist tr", func(e *colly.HTMLElement) {
        sat := ParseStarlinkRow(e)
        if sat != nil {
            *items = append(*items, *sat)
        }
    })

    for _, site := range sites {
        log.Printf("Visiting %q...\n", site)
        c.Visit(site)
    }

    return len(*items)
}

// We define the following table structure we want to parse:
//
// StarlinkSatellites
// ID: int, COSPAR: string, StarlinkName: string, LaunchDate: datetime, Group: string, Revision: string
func InitDB(sqlitePath string) {
    log.Println("Initializing a new DB...")
    os.Remove(sqlitePath)
    db, err := sql.Open("sqlite3", sqlitePath)
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

	sqlStmt := `
	create table StarlinkSatellites (
        ID int primary key,
        COSPAR string,
        StarlinkName string,
        LaunchDate datetime,
        Group string,
        Revision string
    );
	`
    _, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
    log.Println("Created DB in ", sqlitePath)
}

//func ParseNInsert(item ScrapedItem, db *sql.DB) {
//    sqlite3.Version()
//
//    // ----------------------------------
//    // Constellation related transactions
//    // ----------------------------------
//    tx, err := db.Begin()
//	constStmt, err      := tx.Prepare(`insert or ignore into Constellations(name) values(?)`)
//    defer constStmt.Close()
//
//    // Get Constellation ID
//    rows, err := db.Query(fmt.Sprintf(
//        "select * from Constellations where Name = %q", item.Name))
//	if err != nil {
//		log.Fatal(err)
//	}
//    var constellationID int64
//    if rows.Next() {
//		var name string
//		err = rows.Scan(&constellationID, &name)
//        log.Printf("Constellation %q exists - Constellation Id: %d", item.Name, constellationID)
//		if err != nil {
//			log.Panic(err)
//		}
//	} else {
//        log.Println("Creating new Constellation entry...")
//        res, err := constStmt.Exec(item.Name)
//        if err != nil {
//            log.Panic(err)
//        } else {
//            constellationID, _ = res.LastInsertId()
//            log.Printf("Entered Constellation %q - Constellation Id: %d", item.Name, constellationID)
//        }
//    }
//	err = tx.Commit()
//	if err != nil {
//		log.Panic(err)
//	}
//
//    // ----------------------------------
//    // Parse TLE and add Satellite Orbits
//    // ----------------------------------
//    tx, err = db.Begin()
//	satStmt, err        := tx.Prepare(`insert or ignore into Satellites(
//        SATCATID,
//        ConstellationsID
//    ) values(?, ?)`)
//    defer satStmt.Close()
//	satOrbitStmt, err   := tx.Prepare(`insert into SatelliteOrbits(
//        SATCATID,
//        ORBIT_TLE0,
//        ORBIT_TLE1,
//        ORBIT_TLE2,
//        ORBIT_Epoch,
//        ORBIT_InclinationDeg,
//        ORBIT_RAANDeg,
//        ORBIT_AltitudeKm,
//        ORBIT_PeriodS,
//        ORBIT_ElementSetNumber,
//        FILE_Hash,
//        FILE_URL,
//        FILE_LastAccessed
//    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
//	defer satOrbitStmt.Close()
//
//    //log.Println("Parse TLE and add Satellite Orbits")
//    content, err := os.ReadFile(item.TLEFile)
//    if err != nil {
//        log.Fatal(err)
//    }
//    h := sha256.New()
//    h.Write([]byte(content))
//    file_hash := fmt.Sprintf("%x", h.Sum(nil))
//
//    // Check first if there are SatelliteOrbits with this file hash
//    rows, err = db.Query(fmt.Sprintf(
//        "select * from SatelliteOrbits where FILE_Hash = %q", file_hash,))
//	if err != nil {
//		log.Fatal(err)
//	}
//    if !rows.Next() {
//        splitLines := strings.Split(string(content), "\n")
//        var nLines int = len(splitLines) / 3
//        for i := 0; i < nLines; i++ {
//            idx := i * 3
//            orbitData := ParseTle(splitLines[idx], splitLines[idx+1], splitLines[idx+2])
//            //log.Println(orbitData.Epoch.Format("2006-01-02 15:04:05"))
//            //log.Println(srcFileInfo.LastAccessed.Format("2006-01-02 15:04:05"))
//            //log.Printf("New Satellite %d and Database ID %d\n", orbitData.SatcatNum, constellationID)
//            _ = constellationID
//            _, err = satStmt.Exec(
//                orbitData.SatcatNum,
//                constellationID,
//            )
//            if err != nil {
//                log.Panic(err)
//            }
//
//            _, err = satOrbitStmt.Exec(
//                orbitData.SatcatNum,
//                orbitData.TleLine0,
//                orbitData.TleLine1,
//                orbitData.TleLine2,
//                orbitData.Epoch.Format("2006-01-02 15:04:05"),
//                orbitData.Inclination,
//                orbitData.RAAN,
//                orbitData.Altitude,
//                orbitData.Period,
//                orbitData.ElementSetNumber,
//                file_hash,
//                item.URL,
//                time.Now().Format("2006-01-02 15:04:05"),
//            )
//            if err != nil {
//                log.Panic(err)
//            }
//        }
//    } else {
//        log.Printf("Filehash %q for Constellation %q already logged", file_hash, item.Name)
//    }
//    rows.Close()
//
//	err = tx.Commit()
//	if err != nil {
//		log.Panic(err)
//	}
//}

//func AddDB(sqlitePath string, items *[]ScrapedItem) {
//    db, err := sql.Open("sqlite3", "file:" + sqlitePath + "?cache=shared")
//    if err != nil {
//        log.Panic(err)
//    }
//    defer db.Close()
//    for _, item := range *items {
//        ParseNInsert(item, db)
//    }
//}

func main() {
    log.SetOutput(os.Stdout)
    log.Println("Starting Starlink Constellation Scraper...")

    var dbPath string
    flag.StringVar(&dbPath, "d", "scraped.db", "give db path")
	flag.Parse()

    // -----------------------------------------------------------
    // Scrape constellations
    // -----------------------------------------------------------
    items := make([]ScrapedSatellite, 0, 5)
    itemcnt := ScrapeStarlink(&items)
    log.Printf("Finished web scraping. Found %d items\n", itemcnt)

    // -----------------------------------------------------------
    // sqlite3
    // -----------------------------------------------------------
    //if _, err := os.Stat(dbPath); err != nil {
    //    InitDB(dbPath)
    //}
    //AddDB(dbPath, &items)

    log.Println("Finished")
}
