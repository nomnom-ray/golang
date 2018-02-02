package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/kr/pretty"

	"github.com/go-gl/gl/v4.1-core/gl" //needs gcc; msys32 used
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32" //API for creating windows; needs installation
	"github.com/gocarina/gocsv"
	"gopkg.in/cheggaaa/pb.v1"
)

const (
	width            = 1280
	height           = 720
	degRadConversion = math.Pi / 180

	//south-east to north-west
	//lat goes south north; long east west
	latEnd, lngEnd     = 43.45270, -80.49600
	latStart, lngStart = 43.45050, -80.49300
	samplesLat         = 220.0 //must be float
	samplesLng         = 220.0 //must be float

	vertexShaderSource = `
		#version 410
		uniform mat4 projection;
		uniform mat4 camera;
		in vec3 vp;
		void main() {
			gl_Position = vec4(vp, 1.0);
		}
	` + "\x00"

	fragmentShaderSource = `
		#version 410
		out vec4 frag_colour;
		void main() {
			frag_colour = vec4(1.0, 0.0, 0.0, 1.0);
		}
	` + "\x00"
)

type mapVector struct {
	VertX, VertY, VertZ             float32 //always f32 for vertices to opengl
	Latitude, Longtitude, Elevation float64
}

func main() {
	runtime.LockOSThread() //execute in the same OS thread; important for GLFW; always call on init

	window := initGlfw()
	defer glfw.Terminate()

	program, err := initOpenGL()
	if err != nil {
		panic(err)
	}

	// projections(program)

	rightangle, vectorSize := mapMesh()

	vao := makeVao(rightangle, vectorSize)

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	for !window.ShouldClose() {
		draw(vao, window, program, rightangle)
	}
}

func draw(vao uint32, window *glfw.Window, program uint32, rightangle []mapVector) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	//	gl.UseProgram(program)

	// gl.BindVertexArray(vao) //opengl recommends each object having a vao
	// gl.DrawArrays(gl.TRIANGLES, 0, int32(len(rightangle)))
	gl.DrawArrays(gl.POINTS, 0, int32(len(rightangle)))

	glfw.PollEvents()
	window.SwapBuffers() //GLFW does double buf (draw invisible canvas, then swap it to visible canvas when ready)
}

// initGlfw initializes glfw and returns a Window to use.
func initGlfw() *glfw.Window {
	if err := glfw.Init(); err != nil { // initialize the GLFW package
		panic(err)
	}

	//global GLFW properties defining window
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(width, height, "test window", nil, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent() //binding window to current thread

	return window
}

// initOpenGL initializes OpenGL and returns an intiialized program.
func initOpenGL() (uint32, error) {
	if err := gl.Init(); err != nil {
		panic(err)
	}
	version := gl.GoStr(gl.GetString(gl.VERSION))
	log.Println("OpenGL version", version)

	program := gl.CreateProgram()

	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		panic(err)
	}
	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		panic(err)
	}

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program) //gllinkprogram must come after shaders

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	//delete the shader intermediary files after shaders are attached to program
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

// func projections(program uint32) {
// 	gl.UseProgram(program)

// 	projection := mgl32.Perspective(
// 		mgl32.DegToRad(45.0),
// 		float32(width)/height,
// 		0.1,
// 		10.0)
// 	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
// 	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

// 	// fmt.Println(projection)
// 	// fmt.Println(projectionUniform)

// 	camera := mgl32.LookAtV(
// 		mgl32.Vec3{1, 1, 1},
// 		mgl32.Vec3{0, 0, 0},
// 		mgl32.Vec3{0, 1, 0})
// 	cameraUniform := gl.GetUniformLocation(program, gl.Str("camera\x00"))
// 	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

// }

// makeVao tells GPU using opengl what buffer to draw
func makeVao(rightangle []mapVector, vectorSize int32) uint32 { //Vertex Array Object
	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	var vbo uint32                      //vertex buffer object (or just buffer)
	gl.GenBuffers(1, &vbo)              //gen. UUID for 1 vbo and uint ptr to it
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo) //which VRAM (the target) to use for the vbo
	gl.BufferData(
		gl.ARRAY_BUFFER,
		int(vectorSize)*len(rightangle),
		gl.Ptr(rightangle),
		gl.STATIC_DRAW) //filling the buffer with data: 4x4Bytes size; actual data; 2 GL properties

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(
		0,
		3,
		gl.FLOAT,
		false,
		vectorSize,
		nil) //position,color,texture are attributes of vertex; this line defines the attri. layout in buffer

	return vao
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func mapMesh() ([]mapVector, int32) {

	// ellipsoidConfig := ellipsoid.Init(
	// 	"WGS84",
	// 	ellipsoid.Degrees,
	// 	ellipsoid.Meter,
	// 	ellipsoid.LongitudeIsSymmetric,
	// 	ellipsoid.BearingIsSymmetric)

	// LongMeteric, _ := ellipsoidConfig.To(
	// 	latStart,
	// 	lngStart,
	// 	latStart,
	// 	lngEnd)

	// latMeteric, _ := ellipsoidConfig.To(
	// 	latStart,
	// 	lngStart,
	// 	latEnd,
	// 	lngStart)

	fmt.Println("Generating image from downloaded vectors:")

	clientsFile, err := os.Open("resultSouthEastWestNorth3.csv")
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	vectors := []mapVector{}

	if err := gocsv.UnmarshalFile(clientsFile, &vectors); err != nil {
		panic(err)
	}

	projectingProgress := pb.StartNew(int(len(vectors)))

	// cameraX := float32(120.5)
	// cameraZ := float32(147.8)
	// cameraHeight := float32(2.0)
	// groundRef := float32(4.3246)
	// maxVert := float32(math.Max(latMeteric, LongMeteric))
	scaleFactor := float32(1.0)
	sizeMat := mgl32.Scale3D(scaleFactor, scaleFactor, scaleFactor)
	// cmIntervalNorm := 0.01 / maxVert * 100.0
	// cmIntervalNormScaled := cmIntervalNorm * scaleFactor
	// cameraPosition := mgl32.Vec3{cameraX * cmIntervalNormScaled, (cameraHeight + groundRef) * cmIntervalNormScaled, cameraZ * cmIntervalNormScaled}

	cameraPosition := mgl32.Vec3{0.2, 0.2, -0.6}

	cameraViewDirection := mgl32.Vec3{0, 0, 1}
	cameraUp := mgl32.Vec3{0, 1, 0}
	cameraViewDirection = mgl32.QuatRotate(float32(degToRad(0)), cameraUp).Rotate(cameraViewDirection)
	cameraViewDirection = mgl32.QuatRotate(float32(degToRad(0)), cameraViewDirection.Cross(cameraUp)).Rotate(cameraViewDirection)
	translateMat := mgl32.Translate3D(0, 0, 0)
	rotateXMat := mgl32.HomogRotate3DX(float32(degToRad(0)))
	rotateYMat := mgl32.HomogRotate3DY(float32(degToRad(0)))
	rotateZMat := mgl32.HomogRotate3DZ(float32(degToRad(0)))

	// pretty.Println(cameraPosition)

	perspectiveMat := mgl32.Perspective(mgl32.DegToRad(60.0), float32(width)/height, 0.001, 10.0)
	cameraMat := mgl32.LookAtV(
		cameraPosition,                            //position of camera
		(cameraPosition).Add(cameraViewDirection), //direction of view
		cameraUp) //direction of camera orientation

	cameraPerspective := (&perspectiveMat).Mul4(cameraMat).Mul4(sizeMat).Mul4(translateMat).Mul4(rotateXMat).Mul4(rotateYMat).Mul4(rotateZMat)

	pretty.Println(cameraPerspective)

	var vector []mapVector
	for _, vectorelem := range vectors {
		vertex := mgl32.Vec3{vectorelem.VertX, vectorelem.VertY, vectorelem.VertZ}

		perspectiveVector := mgl32.TransformCoordinate(vertex, cameraPerspective)

		vectorelem.VertX = perspectiveVector[0]
		vectorelem.VertY = perspectiveVector[1]
		vectorelem.VertZ = perspectiveVector[2]

		projectingProgress.Increment()
		vector = append(vector, vectorelem)

	}

	clientsFile2, err := os.OpenFile("vector.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	err = gocsv.MarshalFile(&vector, clientsFile2)
	if err != nil {
		panic(err)
	}

	vectorSize := int32(binary.Size(vector[0])) + 4

	return vector, vectorSize

	// var compositeVectorElem mapVector
	// var compositeVector []mapVector

	// for i := 0; i <= int(len(triangle))/3; i += 3 {
	// 	j := i + 1
	// 	k := j + 1
	// 	compositeVectorElem = mapVector{
	// 		vertX:      triangle[i],
	// 		vertY:      triangle[j],
	// 		vertZ:      triangle[k],
	// 		latitude:   0,
	// 		longtitude: 0,
	// 		elevation:  0,
	// 	}
	// 	compositeVector = append(compositeVector, compositeVectorElem)
	// }

	// vectorSize := int32(binary.Size(compositeVectorElem)) + 4

	// pretty.Println(vectorSize)
	// pretty.Println(int(len(compositeVector)))

	// return compositeVector, vectorSize
}

func degToRad(d float64) float64 { return d * degRadConversion }

/*
var (
	triangle = []float32{

	}
)


*/
