package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/kr/pretty"

	"github.com/StefanSchroeder/Golang-Ellipsoid/ellipsoid"
	"github.com/gocarina/gocsv"
	"github.com/nfnt/resize"
	"github.com/nomnom-ray/fauxgl"
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

	scale = 4     // optional supersampling
	fovy  = 30.0  // vertical field of view in degrees
	near  = 0.001 // near clipping plane
	far   = 10.0  // far clipping plane
)

var (
	eye        = fauxgl.V(0, 0, 0)                  // camera position
	center     = fauxgl.V(0, 0, 1)                  // view center position
	up         = fauxgl.V(0, 1, 0)                  // up vector
	light      = fauxgl.V(0.75, 0.5, 1).Normalize() // light direction
	color      = fauxgl.HexColor("#ffb5b5")         // object color
	background = fauxgl.HexColor("#FFF8E3")         // background color
	pickedX    = 1245
	pickedY    = 361
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
	cameraPerspective := cameraModel(maxVert)
	//3D-2D conversion
	triangles, primitiveOnScreen := projection(maxVert, cameraPerspective)

	// pretty.Println("array out of Projection:", primitiveOnScreen)

	if _, ok := rasterPicking(pickedX, pickedY, triangles, primitiveOnScreen, cameraPerspective); ok {
		// pretty.Println(primitiveSelected)
	} else {
		pretty.Println("picking: primitive not selected.")
	}

}

func cameraModel(maxVert float64) fauxgl.Matrix {
	//camera and projection parameters to create a single matrix
	cameraRotationLR := float64(0.0)    //-ve rotates camera clockwise in degrees
	cameraRotationUD := float64(90.0)   //-ve rotates camera downwards in degrees
	cameraX := float64(0.011)           //-ve pans camera to the right
	cameraZ := float64(0.0)             //-ve pans camera to the back
	cameraHeight := float64(0.00002252) //height of the camera from ground
	groundRef := float64(0.0) - 0.02    //ground reference to the lowest ground point in the tile

	cameraPosition := fauxgl.Vector{
		X: cameraX / maxVert,
		Y: (cameraHeight + groundRef) / maxVert,
		Z: cameraZ / maxVert,
	}
	cameraViewDirection := fauxgl.Vector{
		X: 0,
		Y: 0,
		Z: 1,
	}
	cameraUp := fauxgl.Vector{
		X: 0,
		Y: 1,
		Z: 0,
	}
	cameraViewDirection = fauxgl.QuatRotate(
		degToRad(cameraRotationLR), cameraUp).Rotate(cameraViewDirection)
	cameraViewDirection = fauxgl.QuatRotate(
		degToRad(cameraRotationUD), cameraViewDirection.Cross(cameraUp)).Rotate(cameraViewDirection)
	cameraPerspective := fauxgl.LookAt(
		cameraPosition, (cameraPosition).Add(cameraViewDirection), cameraUp).Perspective(
		fovy, imageAspectRatio, near, far)

	return cameraPerspective
}

func projection(maxVert float64, cameraPerspective fauxgl.Matrix) ([]*fauxgl.Triangle, []int) {

	compositeVector := []*mapVector{}
	primitiveIndex := []*mapPrimitiveIndex{}

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

	//constructing a mesh of triangles from index to normalized vertices
	var triangles []*fauxgl.Triangle
	counter := 0.0

	primitiveCounter := 0.0
	for _, index := range primitiveIndex[:5] {
		var triangle fauxgl.Triangle
		for inner := 0; inner < 3; inner++ {
			primitiveCounter = math.Mod(counter, 3)
			if primitiveCounter == 0 {
				triangle.V1.Position = fauxgl.Vector{
					X: compositeVector[index.PrimitiveBottom].VertX,
					Y: compositeVector[index.PrimitiveBottom].VertY,
					Z: compositeVector[index.PrimitiveBottom].VertZ,
				}
				triangle.V1.Texture = fauxgl.Vector{
					X: compositeVector[index.PrimitiveBottom].Latitude,
					Y: compositeVector[index.PrimitiveBottom].Elevation,
					Z: compositeVector[index.PrimitiveBottom].Longtitude,
				}
			} else if primitiveCounter == 1 {
				triangle.V2.Position = fauxgl.Vector{
					X: compositeVector[index.PrimitiveTop].VertX,
					Y: compositeVector[index.PrimitiveTop].VertY,
					Z: compositeVector[index.PrimitiveTop].VertZ,
				}
				triangle.V2.Texture = fauxgl.Vector{
					X: compositeVector[index.PrimitiveTop].Latitude,
					Y: compositeVector[index.PrimitiveTop].Elevation,
					Z: compositeVector[index.PrimitiveTop].Longtitude,
				}
			} else if primitiveCounter == 2 {
				triangle.V3.Position = fauxgl.Vector{
					X: compositeVector[index.PrimitiveLeft].VertX,
					Y: compositeVector[index.PrimitiveLeft].VertY,
					Z: compositeVector[index.PrimitiveLeft].VertZ,
				}
				triangle.V3.Texture = fauxgl.Vector{
					X: compositeVector[index.PrimitiveLeft].Latitude,
					Y: compositeVector[index.PrimitiveLeft].Elevation,
					Z: compositeVector[index.PrimitiveLeft].Longtitude,
				}
			}
			counter++
		}
		triangle.FixNormals()
		triangles = append(triangles, &triangle)
	}
	mesh := fauxgl.NewEmptyMesh()
	triangleMesh := fauxgl.NewTriangleMesh(triangles)
	mesh.Add(triangleMesh)

	//creating the window for CPU render
	contextRender := fauxgl.NewContext(windWidth*scale, windHeight*scale)
	contextRender.SetPickingFlag(false)
	contextRender.ClearColorBufferWith(fauxgl.Transparent)
	// contextRender.ClearDepthBuffer()

	//shading
	shader := fauxgl.NewPhongShader(cameraPerspective, light, eye)
	shader.ObjectColor = color
	contextRender.Shader = shader
	start := time.Now()
	contextRender.DrawMesh(mesh)
	fmt.Println("**********RENDERING**********", time.Since(start), "**********RENDERING**********")

	image := contextRender.Image()
	image = resize.Resize(windWidth, windHeight, image, resize.Bilinear)

	fauxgl.SavePNG("out.png", image)

	return triangles, contextRender.PrimitiveSelectable()
}

func rasterPicking(pickedX, pickedY int, triangles []*fauxgl.Triangle, primitiveOnScreen []int, cameraPerspective fauxgl.Matrix) (int, bool) {

	var trianglesOnScreen []*fauxgl.Triangle

	primitiveOnScreen = sliceUniqMap(primitiveOnScreen)

	// pretty.Println("array in picking:", primitiveOnScreen)
	// pretty.Println("triangles length:", len(triangles))

	if len(primitiveOnScreen) == 1 {
		trianglesOnScreen = append(trianglesOnScreen, triangles[primitiveOnScreen[0]])
	} else if len(primitiveOnScreen) > 1 {
		for _, i := range primitiveOnScreen {
			trianglesOnScreen = append(trianglesOnScreen, triangles[primitiveOnScreen[i]])
		}
	}

	meshOnScreen := fauxgl.NewEmptyMesh()
	triangleMesh := fauxgl.NewTriangleMesh(trianglesOnScreen)
	meshOnScreen.Add(triangleMesh)

	//creating the window for CPU render
	contextPicking := fauxgl.NewContext(windWidth*scale, windHeight*scale)
	contextPicking.SetPickedXY(pickedX*scale, pickedY*scale)
	contextPicking.SetPickingFlag(true)
	contextPicking.SetPrimitiveOnScreen(nil)
	// contextPicking.ClearDepthBuffer()

	//shading
	shader := fauxgl.NewPhongShader(cameraPerspective, light, eye)
	shader.ObjectColor = color
	contextPicking.Shader = shader
	start := time.Now()
	contextPicking.DrawMesh(meshOnScreen)
	fmt.Println("**********PICKING**********", time.Since(start), "**********PICKING**********")

	if contextPicking.PrimitiveOnScreen == nil {
		return 0, false
	}
	return primitiveOnScreen[0], true
}

func sliceUniqMap(s []int) []int {
	seen := make(map[int]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
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

type mapPrimitiveIndex struct {
	PrimitiveBottom, PrimitiveTop, PrimitiveLeft int
}

type triangleID struct {
	Triangle    fauxgl.Triangle
	PrimitiveID int
}
