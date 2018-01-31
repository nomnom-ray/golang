package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"strings"

	"github.com/StefanSchroeder/Golang-Ellipsoid/ellipsoid"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/gocarina/gocsv"
	"github.com/kr/pretty"
	"github.com/mohae/struct2csv"
	"googlemaps.github.io/maps"
	"gopkg.in/cheggaaa/pb.v1"
)

const (
	windWidth        = 512
	windHeight       = 512
	degRadConversion = math.Pi / 180
)

func main() {

	// _, err := os.Stat("result.csv")
	// if err != nil {
	// 	if os.IsNotExist(err) {
	// 		getMapVector(scanner())
	// 	} else {
	// 		log.Fatalf("fatal error: %s", err)
	// 	}
	// }

	projection()
}

func getMapVector(apiKey *string) {

	//south-east to north-west
	//lat goes south north; long east west
	latEnd, lngEnd := 43.4525000, -80.4948000
	latStart, lngStart := 43.4517500, -80.4947990
	samplesLat := 332.0 //must be float
	samplesLng := 2.0   //must be float
	sampleCount := int(samplesLat * samplesLng)

	fmt.Println("Downloading vectors from Google Maps:")

	downloadProgress := pb.StartNew(sampleCount)

	ellipsoidConfig := ellipsoid.Init(
		"WGS84",
		ellipsoid.Degrees,
		ellipsoid.Meter,
		ellipsoid.LongitudeIsSymmetric,
		ellipsoid.BearingIsSymmetric)

	var requestLats []float64
	var requestLngs []float64
	var compositeVector []mapVector
	var compositeVectorElem mapVector

	for i := 0; i < int(samplesLat); i++ {
		y := float64(i)
		requestLats = append(requestLats,
			latStart+(y*(latEnd-latStart))/(samplesLat-1.0))
	}
	for i := 0; i < int(samplesLng); i++ {
		y := float64(i)
		requestLngs = append(requestLngs,
			lngStart+(y*(lngEnd-lngStart))/(samplesLng-1.0))
	}

	clientAccount, err := maps.NewClient(maps.WithAPIKey(strings.TrimSuffix(*apiKey, "\r\n")))
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	for _, requestLat := range requestLats {
		for _, requestLng := range requestLngs {
			r := &maps.ElevationRequest{
				Locations: []maps.LatLng{
					{Lat: requestLat, Lng: requestLng},
				},
			}
			resultVector, err := clientAccount.Elevation(context.Background(), r)
			if err != nil {
				log.Fatalf("fatal error: %s", err)
			}

			xVector, _ := ellipsoidConfig.To(
				requestLat,
				lngStart,
				(*resultVector[0].Location).Lat,
				(*resultVector[0].Location).Lng)

			yVector, _ := ellipsoidConfig.To(
				latStart,
				requestLng,
				(*resultVector[0].Location).Lat,
				(*resultVector[0].Location).Lng)

			compositeVectorElem = mapVector{
				VertX: float64(xVector),
				//90deg on X is flip Y and Z,then -ve nowY; -90deg is flip then -ve nowZ
				VertZ:      float64(yVector),
				VertY:      float64(resultVector[0].Elevation),
				Latitude:   (*resultVector[0].Location).Lat,
				Longtitude: (*resultVector[0].Location).Lng,
				Elevation:  resultVector[0].Elevation,
			}

			compositeVector = append(compositeVector, compositeVectorElem)
			downloadProgress.Increment()
		}
	}

	file, err := os.Create("result.csv")
	if err != nil {
		log.Fatal("can't create file", err)
	}
	defer file.Close()

	writer := struct2csv.NewWriter(file)
	defer writer.Flush()

	err = writer.WriteStructs(compositeVector)
	if err != nil {
		log.Fatal("can't write to file", err)
	}

	downloadProgress.FinishPrint("Vectors downloaded.")
}

func mapModel() {
	//TODO:converting model in meter to model for camera
}

func projection() {

	fmt.Println("Generating image from downloaded vectors:")

	img := image.NewNRGBA(image.Rect(0, 0, windWidth, windHeight))

	clientsFile, err := os.Open("resultProcessed.csv")
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	vectors := []*mapVector{}

	if err := gocsv.UnmarshalFile(clientsFile, &vectors); err != nil {
		panic(err)
	}

	projectingProgress := pb.StartNew(int(len(vectors)))

	sizeMat := mgl64.Scale3D(
		1.0,
		1.0,
		1.0)
	translateMat := mgl64.Translate3D(
		0.0,
		0.0,
		0.0)
	rotateXMat := mgl64.HomogRotate3DX(degToRad(0))
	rotateYMat := mgl64.HomogRotate3DY(degToRad(0))
	rotateZMat := mgl64.HomogRotate3DZ(degToRad(0))
	perspectiveMat := mgl64.Perspective(
		mgl64.DegToRad(30.0),
		float64(windWidth)/windHeight,
		0.1,
		10.0)
	cameraMat := mgl64.LookAtV(
		mgl64.Vec3{0, 0, 3},
		mgl64.Vec3{0, 0, 0},
		mgl64.Vec3{0, 1, 0})
	cameraPerspective := (&perspectiveMat).Mul4(cameraMat).Mul4(sizeMat).Mul4(translateMat).Mul4(rotateXMat).Mul4(rotateYMat).Mul4(rotateZMat)

	for _, vector := range vectors {
		vertex := mgl64.Vec3{vector.VertX, vector.VertY, vector.VertZ}

		// pretty.Println(vertex)

		perspectiveVector := mgl64.TransformCoordinate(vertex, cameraPerspective)

		pretty.Println(perspectiveVector)

		vector.VertX = math.Min(windWidth-1, (perspectiveVector[0]+1)*0.5*windWidth)
		vector.VertY = math.Min(windHeight-1, (1-(perspectiveVector[1]+1)*0.5)*windHeight)
		vector.VertZ = 0

		for y := 0; y < windHeight; y++ {
			for x := 0; x < windWidth; x++ {
				img.Set(int(vector.VertX), int(vector.VertY), color.NRGBA{255, 255, 0, 255})
			}
		}

		projectingProgress.Increment()

	}

	f, err := os.Create("imgTransparentBG.png")
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	projectingProgress.FinishPrint("Image generated.")
}

func scanner() *string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("enter the Google Maps API key")
	fmt.Println("-----------------------------")
	fmt.Print("-> ")
	text, _ := reader.ReadString('\n')
	return &text
}

func degToRad(d float64) float64 { return d * degRadConversion }

type mapVector struct {
	VertX, VertY, VertZ             float64
	Latitude, Longtitude, Elevation float64
}
