package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/mattn/go-sqlite3"
)

type ScrapedSatellite struct {
    COSPAR          string
    StarlinkName    string
    LaunchDate      time.Time
    SGroup          string
    Revision        string
}

// This method requires some domain knowledge about Starlink
// v1.0 consists of Group 1
// v1.5 revision contains Group 2, 3, 4, 5
// v2.m so far, only Group 6
// Satellite Group => SGroup
func ParseStarlinkRow(e *colly.HTMLElement) *ScrapedSatellite {
    sqlite3.Version()

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
            sat.SGroup = splitstr[2][:2]
            sat.Revision = splitstr[1]
        case 1:
            sat.COSPAR = ce.Text
        case 2:
            sat.LaunchDate, _ = time.Parse("02.01.2006", ce.Text)
        }
    })
    if sat.Revision == "v1.0" {
        sat.SGroup = "G1"
    }
    //fmt.Println(e.Index, "Parsed Row:", sat)
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
// ID: int, COSPAR: string, StarlinkName: string, LaunchDate: datetime, SGroup: string, Revision: string
func CreateDB(sqlitePath string, sats *[]ScrapedSatellite) {
    log.Println("Initializing a new DB...")
    os.Remove(sqlitePath)
    db, err := sql.Open("sqlite3", sqlitePath)
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

	sqlStmt := `
	create table StarlinkSatellites (
        ID integer primary key,
        COSPAR string,
        StarlinkName string,
        LaunchDate datetime,
        SGroup string,
        Revision string
    );
	`
    _, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
    log.Println("Created DB in ", sqlitePath)

    // Insert ScrapedSatellite into database
    tx, err := db.Begin()
	if err != nil {
		log.Panic(err)
	}
	constStmt, err := tx.Prepare(`insert or ignore into StarlinkSatellites(COSPAR, StarlinkName, LaunchDate, SGroup, Revision) values(?, ?, ?, ?, ?)`)
    defer constStmt.Close()
	if err != nil {
		log.Panic(err)
	}

    for _, sat := range *sats {
        constStmt.Exec(
            sat.COSPAR,
            sat.StarlinkName,
            sat.LaunchDate.Format("2006-01-02 15:04:05"),
            sat.SGroup,
            sat.Revision,
        )
    }

	err = tx.Commit()
	if err != nil {
		log.Panic(err)
	}
}

func main() {
    log.SetOutput(os.Stdout)
    log.Println("Starting Starlink Constellation Scraper...")

    var dbPath string
    flag.StringVar(&dbPath, "d", "starlink_scraped.db", "give db path")
	flag.Parse()

    // -----------------------------------------------------------
    // Scrape constellations
    // -----------------------------------------------------------
    sats := make([]ScrapedSatellite, 0, 5)
    satcnt := ScrapeStarlink(&sats)
    log.Printf("Finished web scraping. Found %d items\n", satcnt)

    // -----------------------------------------------------------
    // sqlite3
    // -----------------------------------------------------------
    CreateDB(dbPath, &sats)

    log.Println("Finished")
}
