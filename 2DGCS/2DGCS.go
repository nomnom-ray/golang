package main

import (
	"bufio"
	"context"
	"fmt"
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
	latEnd, lngEnd      = 43.45270, -80.49600
	latStart, lngStart  = 43.45050, -80.49300
	sampleResolutionLat = 0.001 //degrees
	sampleResolutionLng = 0.001 //degrees
)

//type declaration is at the very end

//3 main function in main():
// get vector data from Google Maps;
// convert Google maps data to normalized 3D model
// create a 2D image of the 3D model
func main() {

	//web client to get vectors; costs money and slow;
	//client will not run as long as resultRawModel.csv in folder
	_, err := os.Stat("resultVectorModel.csv")
	if err != nil {
		if os.IsNotExist(err) {
			compositeVector, primitiveIndex := getMapVector(scanner())
			primitiveIndexDecoder(compositeVector, primitiveIndex)
		} else {
			log.Fatalf("fatal error: %s", err)
		}
	}

	// create a cartesian model with GCS as units
	maxVert := getModel()

	//3D-2D conversion
	projection(maxVert)

}

func projection(maxVert float64) {

	//TODO: a camera location parser
	// cameraLocation := [...]float64{latStart, lngStart}

	// fmt.Println("Generating image from downloaded vectors:")
	//variable declaration
	// img := image.NewNRGBA(image.Rect(0, 0, windWidth, windHeight))
	compositeVector := []*mapVector{}
	primitiveIndex := []*mapPrimitiveIndex{}
	rasterVectors := []rasterVector{}
	var rasterVectorUnit rasterVector

	//read 3D vector model into struct
	clientsFile, err := os.Open("resultNormModel.csv")
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()
	if err := gocsv.UnmarshalFile(clientsFile, &compositeVector); err != nil {
		panic(err)
	}
	//read 3D vector index into struct
	clientsFile2, err := os.Open("resultPrimativeModel.csv")
	if err != nil {
		panic(err)
	}
	defer clientsFile2.Close()
	if err := gocsv.UnmarshalFile(clientsFile2, &primitiveIndex); err != nil {
		panic(err)
	}

	//normalize 3D model to 1:1:1 camera space
	for i := 0; i <= int(len(compositeVector)-1); i++ {
		compositeVector[i].VertX = compositeVector[i].VertX / maxVert
		compositeVector[i].VertY = compositeVector[i].VertY / maxVert
		compositeVector[i].VertZ = compositeVector[i].VertZ / maxVert
	}

	// projectingProgress := pb.StartNew(int(len(compositeVector)))

	//camera and projection parameters to create a single matrix
	cameraRotationLR := float64(0.0)    //-ve rotates camera clockwise in degrees
	cameraRotationUD := float64(90.0)   //-ve rotates camera downwards in degrees
	cameraX := float64(0.0)             //-ve pans camera to the right in meters
	cameraZ := float64(0.0)             //-ve pans camera to the back in meters
	cameraHeight := float64(0.00002252) //height of the camera from ground in meters
	groundRef := float64(0.0) - 0.005   //ground reference to the lowest ground point in the tile

	cameraPosition := mgl64.Vec3{cameraX / maxVert,
		(cameraHeight + groundRef) / maxVert, cameraZ / maxVert}

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
	cameraPerspective := (&perspectiveMat).Mul4(
		cameraMat).Mul4(translateMat).Mul4(rotateXMat).Mul4(rotateYMat).Mul4(rotateZMat)

	// counter := 0.0
	// primitiveCounter := 0.0

	//loop converts vectors from 3D model, writes to struct for outputing to .csv and .png
	for _, vector := range compositeVector {
		vertex := mgl64.Vec3{vector.VertX, vector.VertY, vector.VertZ}
		perspectiveVector := mgl64.TransformCoordinate(vertex, cameraPerspective)
		vector.VertX = perspectiveVector[0]
		vector.VertY = perspectiveVector[1]
		vector.VertZ = perspectiveVector[2]

		if vector.VertX < -imageAspectRatio || vector.VertX > imageAspectRatio || vector.VertZ < -1 || vector.VertZ > 1 {
			// projectingProgress.Increment()
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
		// for i := 0; i < 5; i++ {
		// 	img.Set(int((rasterVectorUnit).RasterX),
		// 		int((rasterVectorUnit).RasterY)-2+i, color.NRGBA{0, 255, 255, 255})
		// 	img.Set(int((rasterVectorUnit).RasterX)-2+i,
		// 		int((rasterVectorUnit).RasterY), color.NRGBA{0, 255, 255, 255})
		// 	img.Set(int((rasterVectorUnit).RasterX)-1+i,
		// 		int((rasterVectorUnit).RasterY)-1+i, color.NRGBA{0, 255, 255, 255})
		// 	img.Set(int((rasterVectorUnit).RasterX)+1-i,
		// 		int((rasterVectorUnit).RasterY)+1-i, color.NRGBA{0, 0, 0, 255})
		// }
		// pretty.Println("perspective vector:", vector)
		// projectingProgress.Increment()
	}

	//output rasterized vector data
	clientsFile3, err := os.OpenFile("resultPerspectiveModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile3.Close()
	err = gocsv.MarshalFile(&rasterVectors, clientsFile3)
	if err != nil {
		panic(err)
	}
	// f, err := os.Create("imgTransparentBG.png")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// if err := png.Encode(f, img); err != nil {
	// 	f.Close()
	// 	log.Fatal(err)
	// }
	// if err := f.Close(); err != nil {
	// 	log.Fatal(err)
	// }

	// projectingProgress.FinishPrint("Image generated.")
}

func getModel() (maxVert float64) {

	/*
		compositeVector := []*mapVector{}
		//read 3D model into struct
		clientsFile, err := os.Open("resultVectorModel.csv")
		if err != nil {
			panic(err)
		}
		defer clientsFile.Close()
		if err := gocsv.UnmarshalFile(clientsFile, &compositeVector); err != nil {
			panic(err)
		}

		//finds the min/max of ground height, latitude and latitude
		compositeVector[0].VertY = compositeVector[0].Elevation / 1.11 * 0.00001
		minVertY := compositeVector[0].VertY
		maxVertY := compositeVector[0].VertY
		for i := 0; i <= int(len(compositeVector)-1); i++ {
			compositeVector[i].VertY = compositeVector[i].Elevation / 1.11 * 0.00001
			if compositeVector[i].VertY < minVertY {
				minVertY = compositeVector[i].VertY
			}
			if compositeVector[i].VertY > maxVertY {
				maxVertY = compositeVector[i].VertY
			}
		}
		minVertX := math.Abs(compositeVector[0].Latitude)
		minVertZ := math.Abs(compositeVector[0].Longtitude)

		//localize the area using the minimum component of each vector as reference
		for i := 0; i <= int(len(compositeVector)-1); i++ {
			compositeVector[i].VertX = (math.Abs(compositeVector[i].Latitude) - minVertX)
			compositeVector[i].VertY = (compositeVector[i].VertY - minVertY)
			compositeVector[i].VertZ = (math.Abs(compositeVector[i].Longtitude) - minVertZ)
		}
		maxVertX := math.Abs(compositeVector[len(compositeVector)-1].VertX)
		maxVertZ := math.Abs(compositeVector[len(compositeVector)-1].VertZ)
		maxVert = math.Max(math.Max(maxVertX, maxVertZ), maxVertY)

		clientsFile2, err := os.OpenFile("resultNormModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			panic(err)
		}
		defer clientsFile2.Close()
		err = gocsv.MarshalFile(&compositeVector, clientsFile2)
		if err != nil {
			panic(err)
		}

	*/
	return 0.0030232
}

func getMapVector(apiKey *string) ([]*mapVector, []*mapPrimitiveIndex) {

	var compositeVector []*mapVector
	var compositeVectorElem *mapVector
	var primitiveIndex []*mapPrimitiveIndex
	var primitiveIndexElem *mapPrimitiveIndex

	baseLat := 2
	baseLng := int(round(math.Abs((lngEnd - lngStart) / sampleResolutionLng)))
	latHeight := int(round(math.Abs((latEnd - latStart) / sampleResolutionLat)))
	latBaseIndex, lngBaseIndex := 1, 1
	latBaseHeight := latStart + sampleResolutionLat
	latBaseGround := latStart
	vectorIndex := 0.0
	//sometimes the count target is over by baseLng elements, because sampling goes over the boundary
	downloadProgress := pb.StartNew(int((baseLng) * (latHeight - 1 + baseLat)))

	clientAccount, err := maps.NewClient(maps.WithAPIKey(strings.TrimSuffix(*apiKey, "\r\n")))
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	//TODO: hangle case for less than 2 baseLng
	for lngBaseIndex+latBaseIndex <= baseLat+baseLng {
		for lngBaseIndex > 0 && latBaseIndex > 0 && lngBaseIndex <= baseLng && latBaseIndex <= baseLat {

			lngLocation := lngStart + (float64(lngBaseIndex-1)*(lngEnd-lngStart))/(float64(baseLng)-1.0)
			latLocation := latBaseGround + (float64(latBaseIndex-1)*(latBaseHeight-latBaseGround))/(float64(baseLat)-1.0)

			r := &maps.ElevationRequest{
				Locations: []maps.LatLng{
					{Lat: latLocation, Lng: lngLocation},
				},
			}
			baseVector, err := clientAccount.Elevation(context.Background(), r)
			if err != nil {
				log.Fatalf("fatal error: %s", err)
			}

			compositeVectorElem = &mapVector{
				VertX: 0,
				//90deg on X is flip Y and Z,then -ve nowY; -90deg is flip then -ve nowZ
				VertZ:      0,
				VertY:      0,
				Latitude:   (*baseVector[0].Location).Lat,
				Longtitude: (*baseVector[0].Location).Lng,
				Elevation:  baseVector[0].Elevation,
			}

			compositeVector = append(compositeVector, compositeVectorElem)

			primitiveCounter := math.Mod(vectorIndex, 2)

			if primitiveCounter == 1 && len(compositeVector) > 2 {
				primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
				primitiveIndexElem.PrimitiveBottom = int(vectorIndex - 3)
				primitiveIndexElem.PrimitiveTop = int(vectorIndex - 2)
				primitiveIndexElem.PrimitiveLeft = int(vectorIndex - 1)
				primitiveIndex = append(primitiveIndex, primitiveIndexElem)
				primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
				primitiveIndexElem.PrimitiveBottom = int(vectorIndex - 1)
				primitiveIndexElem.PrimitiveTop = int(vectorIndex - 2)
				primitiveIndexElem.PrimitiveLeft = int(vectorIndex - 0)
				primitiveIndex = append(primitiveIndex, primitiveIndexElem)
			}
			downloadProgress.Increment()
			vectorIndex++
			latBaseIndex--
			lngBaseIndex++
		}
		latBaseIndex += lngBaseIndex
		lngBaseIndex = 1
		if latBaseIndex >= baseLat {
			lngBaseIndex += latBaseIndex - baseLat
			latBaseIndex = baseLat
		}
	}
	vectorIndex--

	if 1 <= latHeight-1 {
		for latTier := 0; latTier <= latHeight-baseLat; latTier += 2 {
			for assignedIndex, assignedVector := range compositeVector[(latTier)*baseLng : (latTier+2)*baseLng] {
				if odd(assignedIndex) {

					r := &maps.ElevationRequest{
						Locations: []maps.LatLng{
							{Lat: assignedVector.Latitude + sampleResolutionLat, Lng: assignedVector.Longtitude},
						},
					}
					baseVector, err := clientAccount.Elevation(context.Background(), r)
					if err != nil {
						log.Fatalf("fatal error: %s", err)
					}
					compositeVectorElem = &mapVector{
						VertX:      0,
						VertZ:      0,
						VertY:      0,
						Latitude:   (*baseVector[0].Location).Lat,
						Longtitude: (*baseVector[0].Location).Lng,
						Elevation:  baseVector[0].Elevation,
					}
					compositeVector = append(compositeVector, compositeVectorElem)

					r = &maps.ElevationRequest{
						Locations: []maps.LatLng{
							{Lat: assignedVector.Latitude + sampleResolutionLat*2, Lng: assignedVector.Longtitude},
						},
					}
					baseVector, err = clientAccount.Elevation(context.Background(), r)
					if err != nil {
						log.Fatalf("fatal error: %s", err)
					}
					compositeVectorElem = &mapVector{
						VertX:      0,
						VertZ:      0,
						VertY:      0,
						Latitude:   (*baseVector[0].Location).Lat,
						Longtitude: (*baseVector[0].Location).Lng,
						Elevation:  baseVector[0].Elevation,
					}
					compositeVector = append(compositeVector, compositeVectorElem)

					primitiveCounterTiers := math.Mod(float64(latTier+1), 2)
					indexBoundaryLng := ((latTier-latTier/2-1)+2)*baseLng*2 + 2
					loopTierCounter := ((latTier - latTier/2 - 1) + 1) * baseLng * 2

					if primitiveCounterTiers == 1 && len(compositeVector) > indexBoundaryLng {
						primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
						primitiveIndexElem.PrimitiveBottom = int(assignedIndex + loopTierCounter - 2)
						primitiveIndexElem.PrimitiveTop = int(vectorIndex - 1)
						primitiveIndexElem.PrimitiveLeft = int(assignedIndex + loopTierCounter)
						primitiveIndex = append(primitiveIndex, primitiveIndexElem)
						primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
						primitiveIndexElem.PrimitiveBottom = int(assignedIndex + loopTierCounter)
						primitiveIndexElem.PrimitiveTop = int(vectorIndex - 1)
						primitiveIndexElem.PrimitiveLeft = int(vectorIndex + 1)
						primitiveIndex = append(primitiveIndex, primitiveIndexElem)
					}
					vectorIndex += 2

					if assignedIndex == baseLng*2-1 {
						for i := 0; baseLng-1 > i; i++ {
							primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
							primitiveIndexElem.PrimitiveBottom = int(vectorIndex) - (baseLng*2 - 1) + i*2
							primitiveIndexElem.PrimitiveTop = int(vectorIndex) - (baseLng*2 - 2) + i*2
							primitiveIndexElem.PrimitiveLeft = int(vectorIndex) - (baseLng*2 - 3) + i*2
							primitiveIndex = append(primitiveIndex, primitiveIndexElem)
							primitiveIndexElem = &mapPrimitiveIndex{0, 0, 0}
							primitiveIndexElem.PrimitiveBottom = int(vectorIndex) - (baseLng*2 - 3) + i*2
							primitiveIndexElem.PrimitiveTop = int(vectorIndex) - (baseLng*2 - 2) + i*2
							primitiveIndexElem.PrimitiveLeft = int(vectorIndex) - (baseLng*2 - 4) + i*2
							primitiveIndex = append(primitiveIndex, primitiveIndexElem)
						}
					}
					downloadProgress.Increment()
					downloadProgress.Increment()
				}

			}

		}
	}

	clientsFile, err := os.OpenFile("resultVectorModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	err = gocsv.MarshalFile(&compositeVector, clientsFile)
	if err != nil {
		panic(err)
	}

	clientsFile2, err := os.OpenFile("resultPrimativeModel.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile2.Close()

	err = gocsv.MarshalFile(&primitiveIndex, clientsFile2)
	if err != nil {
		panic(err)
	}

	downloadProgress.FinishPrint("Vectors downloaded.")
	return compositeVector, primitiveIndex
}

func scanner() *string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("enter the Google Maps API key")
	fmt.Println("-----------------------------")
	fmt.Print("-> ")
	text, _ := reader.ReadString('\n')
	return &text
}

func primitiveIndexDecoder(compositeVector []*mapVector, primitiveIndex []*mapPrimitiveIndex) {

	// pretty.Println(compositeVector[len(compositeVector)-1])
	// pretty.Println(compositeVector)
	// pretty.Println(primitiveIndex[:])

	// for _, index := range primitiveIndex {
	// 	pretty.Println(
	// 		compositeVector[index.PrimitiveBottom].Latitude,
	// 		compositeVector[index.PrimitiveBottom].Longtitude,
	// 		compositeVector[index.PrimitiveBottom].Elevation)
	// 	pretty.Println(
	// 		compositeVector[index.PrimitiveTop].Latitude,
	// 		compositeVector[index.PrimitiveTop].Longtitude,
	// 		compositeVector[index.PrimitiveTop].Elevation)
	// 	pretty.Println(
	// 		compositeVector[index.PrimitiveLeft].Latitude,
	// 		compositeVector[index.PrimitiveLeft].Longtitude,
	// 		compositeVector[index.PrimitiveLeft].Elevation)
	// }
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

func degToRad(d float64) float64 { return d * degRadConversion }

func odd(number int) bool { return number%2 != 0 }

func round(f float64) int {
	if math.Abs(f) < 0.5 {
		return 0
	}
	return int(f + math.Copysign(0.5, f))
}

type mapVector struct {
	VertX, VertY, VertZ             float64
	Latitude, Longtitude, Elevation float64
}

type rasterVector struct {
	RasterX, RasterY uint32
	mapVector
}

type mapPrimitiveIndex struct {
	PrimitiveBottom, PrimitiveTop, PrimitiveLeft int
}
