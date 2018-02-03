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
	"googlemaps.github.io/maps"
	"gopkg.in/cheggaaa/pb.v1"
)

const (
	windWidth        = 1280.0
	windHeight       = 720.0
	degRadConversion = math.Pi / 180
	imageAspectRatio = windWidth / windHeight

	//south-east to north-west
	//lat goes south north; long east west
	latEnd, lngEnd     = 43.45270, -80.49600
	latStart, lngStart = 43.45050, -80.49300
	samplesLat         = 220.0 //must be float
	samplesLng         = 220.0 //must be float
)

//type declaration is at the very end

//3 main function in main():
// get vector data from Google Maps;
// convert Google maps data to normalized 3D model
// create a 2D image of the 3D model
func main() {

	//data for getModel(); only maxVert(max boundary) used
	_, _, maxVert := mapBoundary()

	//web client to get vectors; costs money and slow;
	//client will not run as long as resultRawModel.csv in folder
	_, err := os.Stat("resultRawModel.csv")
	if err != nil {
		if os.IsNotExist(err) {
			compositeVector := getMapVector(scanner())
			getModel(compositeVector, maxVert)
		} else {
			log.Fatalf("fatal error: %s", err)
		}
	}

	//3D-2D conversion: uses model .csv file from getModel()
	projection(maxVert)

}

func projection(maxVert float64) {

	//variable declaration
	fmt.Println("Generating image from downloaded vectors:")
	img := image.NewNRGBA(image.Rect(0, 0, windWidth, windHeight))
	vectors := []*mapVector{}
	rasterVectors := []rasterVector{}
	var rasterVectorUnit rasterVector
	projectingProgress := pb.StartNew(int(len(vectors)))

	//read 3D model into struct
	clientsFile, err := os.Open("resultNormModel.csv")
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()
	if err := gocsv.UnmarshalFile(clientsFile, &vectors); err != nil {
		panic(err)
	}

	//camera and projection parameters to create a single matrix
	cameraRotationLR := 45.0  //-ve rotates camera clockwise in degrees
	cameraRotationUD := -20.0 //-ve rotates camera downwards in degrees
	cameraX := (-53.3)        //-ve pans camera to the right in meters
	cameraZ := (-43.3)        //-ve pans camera to the back in meters
	cameraHeight := (2.5)     //height of the camera from ground in meters
	groundRef := (63.3)       //ground reference to the lowest ground point in the tile
	scaleFactor := (1.0)
	sizeMat := mgl64.Scale3D(scaleFactor, scaleFactor, scaleFactor)
	cmIntervalNorm := 0.01 / maxVert * 100.0
	cmIntervalNormScaled := cmIntervalNorm * scaleFactor
	cameraPosition := mgl64.Vec3{cameraX * cmIntervalNormScaled,
		(cameraHeight + groundRef) * cmIntervalNormScaled, cameraZ * cmIntervalNormScaled}

	cameraViewDirection := mgl64.Vec3{0, 0, 1}
	cameraUp := mgl64.Vec3{0, 1, 0}
	cameraViewDirection = mgl64.QuatRotate(
		degToRad(cameraRotationLR), cameraUp).Rotate(cameraViewDirection)
	cameraViewDirection = mgl64.QuatRotate(
		degToRad(cameraRotationUD), cameraViewDirection.Cross(cameraUp)).Rotate(cameraViewDirection)

	translateMat := mgl64.Translate3D(0, 0, 0)
	rotateXMat := mgl64.HomogRotate3DX(degToRad(0))
	rotateYMat := mgl64.HomogRotate3DY(degToRad(0))
	rotateZMat := mgl64.HomogRotate3DZ(degToRad(0))

	perspectiveMat := mgl64.Perspective(mgl64.DegToRad(90.0), imageAspectRatio, 0.001, 10.0)
	cameraMat := mgl64.LookAtV(
		cameraPosition,                            //position of camera
		(cameraPosition).Add(cameraViewDirection), //direction of view
		cameraUp) //direction of camera orientation

	//cameraPerspective: the matrix for 3D-2D vector conversion
	cameraPerspective := (&perspectiveMat).Mul4(cameraMat).Mul4(
		sizeMat).Mul4(translateMat).Mul4(rotateXMat).Mul4(rotateYMat).Mul4(rotateZMat)

	//loop converts vectors from 3D model, writes to struct for outputing to .csv and .png
	for _, vector := range vectors {
		vertex := mgl64.Vec3{vector.VertX, vector.VertY, vector.VertZ}
		perspectiveVector := mgl64.TransformCoordinate(vertex, cameraPerspective)
		vector.VertX = perspectiveVector[0]
		vector.VertY = perspectiveVector[1]
		vector.VertZ = perspectiveVector[2]

		if vector.VertX < -imageAspectRatio || vector.VertX > imageAspectRatio || vector.VertZ < -1 || vector.VertZ > 1 {
			projectingProgress.Increment()
			continue
		}
		if windWidth > uint32((perspectiveVector[0]+1)*0.5*windWidth) {
			rasterVectorUnit.RasterX = uint32((perspectiveVector[0] + 1) * 0.5 * windWidth)
		} else {
			rasterVectorUnit.RasterX = windWidth
		}
		if windHeight > uint32((1-(perspectiveVector[1]+1)*0.5)*windHeight) {
			rasterVectorUnit.RasterY = uint32((1 - (perspectiveVector[1]+1)*0.5) * windHeight)
		} else {
			rasterVectorUnit.RasterY = windHeight
		}
		rasterVectors = append(rasterVectors, rasterVectorUnit)

		//create more color pixels around each vector pixel to make them look bigger
		for i := 0; i < 5; i++ {
			img.Set(int((rasterVectorUnit).RasterX),
				int((rasterVectorUnit).RasterY)-2+i, color.NRGBA{0, 255, 255, 255})
			img.Set(int((rasterVectorUnit).RasterX)-2+i,
				int((rasterVectorUnit).RasterY), color.NRGBA{0, 255, 255, 255})
			img.Set(int((rasterVectorUnit).RasterX)-1+i,
				int((rasterVectorUnit).RasterY)-1+i, color.NRGBA{0, 255, 255, 255})
			img.Set(int((rasterVectorUnit).RasterX)+1-i,
				int((rasterVectorUnit).RasterY)+1-i, color.NRGBA{0, 0, 0, 255})
		}
		// pretty.Println("perspective vector:", vector)
		projectingProgress.Increment()
	}

	//output rasterized vector data
	clientsFile2, err := os.OpenFile("vectors.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()
	err = gocsv.MarshalFile(&rasterVectors, clientsFile2)
	if err != nil {
		panic(err)
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

func getModel(compositeVector []*mapVector, maxVert float64) {

	minVertY := compositeVector[0].VertY

	//assuming (ground dimension > elevation) with ground as reference plane
	// maxVert := math.Max(maxVertX, maxVertZ)

	for i := 0; i <= int(len(compositeVector))-1; i++ {
		if compositeVector[i].VertY < minVertY {
			minVertY = compositeVector[i].VertY
		}
	}
	for i := 0; i <= int(len(compositeVector))-1; i++ {
		compositeVector[i].VertX = (compositeVector[i].VertX) / maxVert
		compositeVector[i].VertY = (compositeVector[i].VertY - minVertY) / maxVert
		compositeVector[i].VertZ = compositeVector[i].VertZ / maxVert
	}

	clientsFile, err := os.OpenFile("resultNormModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()
	err = gocsv.MarshalFile(&compositeVector, clientsFile)
	if err != nil {
		panic(err)
	}
}

func mapBoundary() (float64, float64, float64) {
	ellipsoidConfig := ellipsoid.Init(
		"WGS84",
		ellipsoid.Degrees,
		ellipsoid.Meter,
		ellipsoid.LongitudeIsSymmetric,
		ellipsoid.BearingIsSymmetric)

	xDistance, _ := ellipsoidConfig.To(
		latStart,
		lngStart,
		latStart,
		lngEnd)

	yDistance, _ := ellipsoidConfig.To(
		latStart,
		lngStart,
		latEnd,
		lngStart)

	return yDistance, xDistance, math.Max(xDistance, yDistance)
}

func scanner() *string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("enter the Google Maps API key")
	fmt.Println("-----------------------------")
	fmt.Print("-> ")
	text, _ := reader.ReadString('\n')
	return &text
}

func getMapVector(apiKey *string) []*mapVector {

	fmt.Println("Downloading vectors from Google Maps:")

	downloadProgress := pb.StartNew(int(samplesLat * samplesLng))

	ellipsoidConfig := ellipsoid.Init(
		"WGS84",
		ellipsoid.Degrees,
		ellipsoid.Meter,
		ellipsoid.LongitudeIsSymmetric,
		ellipsoid.BearingIsSymmetric)

	var requestLats []float64
	var requestLngs []float64
	var compositeVector []*mapVector
	var compositeVectorElem *mapVector

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

			compositeVectorElem = &mapVector{
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

	clientsFile, err := os.OpenFile("resultRawModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	err = gocsv.MarshalFile(&compositeVector, clientsFile)
	if err != nil {
		panic(err)
	}

	downloadProgress.FinishPrint("Vectors downloaded.")

	return compositeVector
}

func degToRad(d float64) float64 { return d * degRadConversion }

type mapVector struct {
	VertX, VertY, VertZ             float64
	Latitude, Longtitude, Elevation float64
}

type rasterVector struct {
	RasterX, RasterY uint32
	mapVector
}
