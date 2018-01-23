package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw" //API for creating windows, contexts and surfaces...
	"github.com/go-gl/mathgl/mgl32"
)

// projection * camera *
const (
	width  = 500
	height = 500

	vertexShaderSource = `
		#version 410
		uniform mat4 projection;
		uniform mat4 camera;
		in vec3 vp;
		void main() {
			gl_Position = projection * camera * vec4(vp, 1.0);
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

func main() {
	runtime.LockOSThread() //execute in the same OS thread; important for GLFW; always call on init

	window := initGlfw()
	defer glfw.Terminate()

	program, err := initOpenGL()
	if err != nil {
		panic(err)
	}

	projections(program)

	vao := makeVao(triangle)

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	for !window.ShouldClose() {
		draw(vao, window, program)
	}
}

func draw(vao uint32, window *glfw.Window, program uint32) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	gl.UseProgram(program)

	gl.BindVertexArray(vao) //opengl recommends each object having a vao
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(triangle)/3))

	glfw.PollEvents()
	window.SwapBuffers() //GLFW does double buf (draw invisible canvas, then swap it to visible canvas when ready)
}

var (
	triangle = []float32{ //always f32 for vertices to opengl
		-0.5, 0.5, 0, // top (x,y,z) of the window between (-1, 1)
		-0.5, -0.5, 0, // left
		0.5, -0.5, -0.5, // right; -ve Z pivots away from camera
	}
)

// initGlfw initializes glfw and returns a Window to use.
func initGlfw() *glfw.Window {
	if err := glfw.Init(); err != nil { // initialize the GLFW package
		panic(err)
	}

	//global GLFW properties defining window
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4) // OR 2
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

func projections(program uint32) {
	gl.UseProgram(program)

	projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/height, 0.1, 10.0)
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	camera := mgl32.LookAtV(mgl32.Vec3{0, 0, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// model := mgl32.Ident4()
	// modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	// gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])
}

// makeVao tells GPU using opengl what buffer to draw
func makeVao(points []float32) uint32 { //Vertex Array Object
	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	var vbo uint32                                                                //vertex buffer object (or just buffer)
	gl.GenBuffers(1, &vbo)                                                        //gen. UUID for 1 vbo and uint ptr to it
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)                                           //which VRAM (the target) to use for the vbo
	gl.BufferData(gl.ARRAY_BUFFER, 4*len(points), gl.Ptr(points), gl.STATIC_DRAW) //filling the buffer with data: 4x4Bytes size; actual data; 2 GL properties

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, int32(len(triangle)*4/3), nil) //position,color,texture are attributes of vertex; this line defines the attri. layout in buffer

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
