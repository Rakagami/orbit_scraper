# Orbit Scraper 

This script scrapes current constellation data form celestrak supplemental data and stores them into a Sqlite3 database.

## Tables

*Constellations*

| ID  | Name   |
|-------------- | ------- |
| _Incrementally incresing ID_    | _Name of Constellation_     |

*Satellites*

| SATCATID  | ConstellationsID   | LaunchDate   |
|-------------- | -------------- | -------------- |
| SATCAT ID    | Foreign Key, Constellation ID     | Launch Date (not yet implemented)  |

*SatelliteOrbits*

| ID  | SATCATID   | ORBIT_TLE0   | ORBIT_TLE1   | ORBIT_TLE2   | ORBIT_Epoch | ORBIT_InclinationDeg | ORBIT_RAANDeg | ORBIT_AltitudeKm | ORBIT_PeriodS | ORBIT_ElementSetNumber | FILE_Hash | FILE_URL | FILE_LastAccessed |
|--- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
|--- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

The values used scraped of the TLEs is intended for LEO satellites, which is why only inclination, raan, and altitude is scraped. To get actualy ECI position or groundtrack coordinates we also need to look at the anomalies, but that is a feature for another time.
Some notes on the columns:

- altitude km is the semi-major axis minus earth radis
- File hash is the hash of the file from celestrak which contains the TLEs

## Usage

Either compile the code or run:

```
go run scraper.go -d /path/to/your/db/file
```

The sqlite database file will be created if it doesn't yet exists
