package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var stops, stopTimes, routes, trips, shapes GTFSTable

var route_types map[int]string = map[int]string{
	0:  "Tram, Streetcar, Light rail. Any light rail or street level system within a metropolitan area.",
	1:  "Subway, Metro. Any underground rail system within a metropolitan area.",
	2:  "Rail. Used for intercity or long-distance travel.",
	3:  "Bus. Used for short- and long-distance bus routes.",
	4:  "Ferry. Used for short- and long-distance boat service.",
	5:  "Cable tram. Used for street-level rail cars where the cable runs beneath the vehicle (e.g., cable car in San Francisco).",
	6:  "Aerial lift, suspended cable car (e.g., gondola lift, aerial tramway). Cable transport where cabins, cars, gondolas or open chairs are suspended by means of one or more cables.",
	7:  "Funicular. Any rail system designed for steep inclines.",
	11: "Trolleybus. Electric buses that draw power from overhead wires using poles.",
	12: "Monorail. Railway in which the track consists of a single rail or a beam.",
}

var routeTypes []string = []string{}

type GTFSTable struct {
	header map[string]int
	values []map[int]string
}

func (t GTFSTable) printValues() {
	for _, value := range t.values {
		for hColumn, hIndex := range t.header {
			fmt.Println(hColumn, " ->", value[hIndex])
		}
		fmt.Println("")
	}
}

func (t GTFSTable) find(headerIndex int, value string) (map[int]string, error) {
	for _, v := range t.values {
		if v[headerIndex] == value {
			return v, nil
		}
	}
	return nil, errors.New("cannot find")
}

type GeoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

type GeoJSONFeature struct {
	Type       string          `json:"type"`
	Geometry   GeoJSONGeometry `json:"geometry"`
	Properties interface{}     `json:"properties"`
}

type GeoJSONCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

func main() {
	routeTypesCSV := flag.String("route_types", "", "Comma-separated list of route types")

	// Parse the command-line flags
	flag.Parse()

	// Split the CSV string into a slice of strings
	routeTypes = strings.Split(*routeTypesCSV, ",")

	var wg sync.WaitGroup

	// Add two to the wait group counter
	wg.Add(2)

	err := populateGTFS()
	if err != nil {
		fmt.Println(err)
	}

	go func() {
		defer wg.Done()
		stopsErr := generateStopGeoJSON()
		if stopsErr != nil {
			fmt.Println(err)
		}

	}()

	go func() {
		defer wg.Done()
		routesErr := generateRoutesGeoJSON()
		if routesErr != nil {
			fmt.Println(routesErr)
		}
	}()

	wg.Wait()

	fmt.Println("Both GeoJSON generation tasks are complete.")
}

func getTextFileLines(path string) ([]string, error) {
	var lines []string
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lines, nil
}

func parseColumns(line string) map[string]int {
	parsedColumns := make(map[string]int)

	columns := strings.Split(line, ",")

	for i, column := range columns {
		parsedColumns[column] = i
	}

	return parsedColumns
}

func getGTFSData(path string) (GTFSTable, error) {
	var data []map[int]string
	lineData, err := getTextFileLines(path)
	if err != nil {
		return GTFSTable{}, err
	}

	columns := parseColumns(lineData[0])

	for i, line := range lineData {
		if i == 0 {
			continue
		}

		mapData := make(map[int]string)

		values := strings.Split(line, ",")

		for j, value := range values {
			mapData[j] = value
		}

		data = append(data, mapData)

	}

	table := GTFSTable{
		header: columns,
		values: data,
	}

	return table, nil
}

func populateGTFS() error {
	_stops, err := getGTFSData("GTFS/stops.txt")
	if err != nil {
		fmt.Println(err)
		return err
	}
	stops = _stops

	_stopTimes, err := getGTFSData("GTFS/stop_times.txt")
	if err != nil {
		fmt.Println(err)
		return err
	}
	stopTimes = _stopTimes

	_routes, err := getGTFSData("GTFS/routes.txt")
	if err != nil {
		fmt.Println(err)
		return err
	}
	routes = _routes

	_trips, err := getGTFSData("GTFS/trips.txt")
	if err != nil {
		fmt.Println(err)
		return err
	}
	trips = _trips

	_shapes, err := getGTFSData("GTFS/shapes.txt")
	if err != nil {
		fmt.Println(err)
		return err
	}
	shapes = _shapes

	return nil
}

func generateRoutesGeoJSON() error {
	rRouteIdIndex := routes.header["route_id"]
	rRouteNameIndex := routes.header["route_long_name"]
	rRouteColorIndex := routes.header["route_color"]

	tTripShapeIdIndex := trips.header["shape_id"]
	tRouteIdIndex := trips.header["route_id"]

	shShapeIdIndex := shapes.header["shape_id"]
	shLatIndex := shapes.header["shape_pt_lat"]
	shLonIndex := shapes.header["shape_pt_lon"]

	features := []GeoJSONFeature{}

	for i, rValues := range routes.values {
		routeId := rValues[rRouteIdIndex]
		for _, tValues := range trips.values {
			if routeId == tValues[tRouteIdIndex] {
				shapeId := tValues[tTripShapeIdIndex]
				tripCoordinates := [][]float64{}
				for _, shValues := range shapes.values {
					if shapeId == shValues[shShapeIdIndex] {
						shapeLat, ok := shValues[shLatIndex]
						if !ok {
							shapeLat = "0.0"
						}
						shapeLatF, err := strconv.ParseFloat(shapeLat, 64)
						if err != nil {
							shapeLatF = 0.0
						}
						shapeLon, ok := shValues[shLonIndex]
						if !ok {
							shapeLon = "0.0"
						}
						shapeLonF, err := strconv.ParseFloat(shapeLon, 64)
						if err != nil {
							shapeLonF = 0.0
						}
						shapeCoordinates := []float64{shapeLatF, shapeLonF}
						tripCoordinates = append(tripCoordinates, shapeCoordinates)
					}
				}
				feature := GeoJSONFeature{
					Type: "Feature",
					Geometry: GeoJSONGeometry{
						Type:        "MultiLineString",
						Coordinates: tripCoordinates,
					},
					Properties: map[string]interface{}{
						"shape_id":        shapeId,
						"route_color":     rValues[rRouteColorIndex],
						"route_long_name": rValues[rRouteNameIndex],
						"route_id":        routeId,
					},
				}

				if isMapInSlice(feature, features) {
					continue
				}
				features = append(features, feature)
			}
		}
		percentComplete := int(float32(i) / float32(len(routes.values)) * 100)
		fmt.Println(i, rValues[rRouteNameIndex], "complete ->", percentComplete, "percent complete")
	}

	routesCollection := GeoJSONCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	jsonData, err := json.Marshal(routesCollection)
	if err != nil {
		fmt.Println("Error converting to JSON:", err)
	}

	jsonErr := writeJSON(string(jsonData), "output/map-routes-data.geojson")
	if err != nil {
		return jsonErr
	}

	fmt.Println("Created output/map-routes-data.geojson")

	return nil
}

func generateStopGeoJSON() error {
	sStopIdIndex := stops.header["stop_id"]
	sStopNameIndex := stops.header["stop_name"]
	sStopLatIndex := stops.header["stop_lat"]
	sStopLonIndex := stops.header["stop_lon"]

	stStopIdIndex := stopTimes.header["stop_id"]
	stTripIdIndex := stopTimes.header["trip_id"]

	tTripIdIndex := trips.header["trip_id"]
	tRouteIdIndex := trips.header["route_id"]

	rTripIdIndex := routes.header["trip_id"]
	rRouteIdIndex := routes.header["route_id"]
	rRouteNameIndex := routes.header["route_long_name"]
	rRouteColorIndex := routes.header["route_color"]
	rRouteTypeIndex := routes.header["route_type"]

	features := []GeoJSONFeature{}

stopLoop:
	for i, sValue := range stops.values {
		for _, stValue := range stopTimes.values {
			if sValue[sStopIdIndex] == stValue[stStopIdIndex] {
				trip, err := trips.find(tTripIdIndex, stValue[stTripIdIndex])
				if err != nil {
					continue
				}
				route, err := routes.find(rTripIdIndex, trip[tRouteIdIndex])
				if err != nil {
					continue
				}
				if len(routeTypes) > 0 && !includes(routeTypes, route[rRouteTypeIndex]) {
					continue stopLoop
				}
				stopLat, ok := sValue[sStopLatIndex]
				if !ok {
					stopLat = "0.0"
				}
				stopLatF, err := strconv.ParseFloat(stopLat, 64)
				if err != nil {
					stopLatF = 0.0
				}
				stopLon, ok := sValue[sStopLonIndex]
				if !ok {
					stopLon = "0.0"
				}
				stopLonF, err := strconv.ParseFloat(stopLon, 64)
				if err != nil {
					stopLonF = 0.0
				}
				feature := GeoJSONFeature{
					Type: "Feature",
					Geometry: GeoJSONGeometry{
						Type:        "Point",
						Coordinates: []float64{stopLatF, stopLonF},
					},
					Properties: map[string]interface{}{
						"stop_name":       sValue[sStopNameIndex],
						"stop_id":         sValue[sStopIdIndex],
						"route_long_name": route[rRouteNameIndex],
						"route_id":        route[rRouteIdIndex],
						"route_color":     route[rRouteColorIndex],
					},
				}
				if isMapInSlice(feature, features) {
					continue
				}
				features = append(features, feature)
			}
		}
		percentComplete := int(float32(i) / float32(len(stops.values)) * 100)
		fmt.Println(i, sValue[sStopNameIndex], "complete ->", percentComplete, "percent complete")
	}

	stopCollection := GeoJSONCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	jsonData, err := json.Marshal(stopCollection)
	if err != nil {
		fmt.Println("Error converting to JSON:", err)
	}

	jsonErr := writeJSON(string(jsonData), "output/map-stops-data.geojson")
	if err != nil {
		return jsonErr
	}

	fmt.Println("Created output/map-stops-data.geojson")
	return nil
}

func isMapInSlice(target GeoJSONFeature, slice []GeoJSONFeature) bool {
	for _, value := range slice {
		if reflect.DeepEqual(target, value) {
			return true
		}
	}
	return false
}

func writeJSON(json string, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return err
	}
	defer file.Close()

	_, err = file.WriteString(json)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return err
	}

	return nil
}

func includes[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
