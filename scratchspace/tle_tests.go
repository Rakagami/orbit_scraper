package main

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

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


func main() {
    //tle0 := "STARLINK-1007"
    //tle1 := "1 44713C 19074A   23053.20743056  .00024213  00000+0  16238-2 0   534"
    //tle2 := "2 44713  53.0505 136.2602 0001443  97.6224 213.8633 15.06341972    16"

    tle0 := "ISS (ZARYA)"
    tle1 := "1 25544U 98067A   08264.51782528 -.00002182  00000-0 -11606-4 0  2927"
    tle2 := "2 25544  51.6416 247.4627 0006703 130.5360 325.0288 15.72125391563537"


    MU   := 398600.4418 // unit: (km)^3 / s^2
    EARTH_R := 6371.0 // unit: km

    _ = tle0
    _ = tle1
    _ = tle2

    fmt.Println("SatcatNum"        , tle1[2:7])
    fmt.Println("Epoch"            , ParseEpoch(tle1[18:32]))
    fmt.Println("Inclination[deg]" , tle2[8:16])
    fmt.Println("RAAN[deg]"        , tle2[17:25])
    mm, _ := strconv.ParseFloat(tle2[52:63], 32)
    fmt.Println("MeanMotion[rev/day]", mm)

    period_s := 86400.0 / mm
    fmt.Println("Period[s]", period_s)
    smja := math.Pow((math.Sqrt(MU) * period_s) / (2.0*math.Pi), 2.0/3.0)

    fmt.Println("Altitude[km]"         , smja - EARTH_R)
}
