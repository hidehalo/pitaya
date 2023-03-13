package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/loader/gltf"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/util/helper"
	"github.com/g3n/engine/window"
	// gltfWrap "github.com/topfreegames/pitaya/v2/examples/demo/cluster/resouces/gltf"
)

func main() {
	absFilePath, err := filepath.Abs("./Duck.glb")
	if err != nil {
		panic(err)
	}
	gltfDoc, err := gltf.ParseBin(absFilePath)
	if err != nil {
		panic(err)
	}
	fmt.Println("Nodes=", len(gltfDoc.Nodes))
	fmt.Println("Scenes=", len(gltfDoc.Scenes))
	fmt.Println("Cameras=", len(gltfDoc.Cameras))
	fmt.Println("Animations=", len(gltfDoc.Animations))
	// anims := make([]*animation.Animation, 0)
	// for _, a := range gltfDoc.Animations {
	// 	fmt.Println("Animation", a.Name)
	// 	anim, err := gltfDoc.LoadAnimationByName(a.Name)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	anims = append(anims, anim)
	// }
	fmt.Println("Buffers=", len(gltfDoc.Buffers))
	fmt.Println("BufferViews=", len(gltfDoc.BufferViews))
	fmt.Println("Extensions=", len(gltfDoc.Extensions))
	for _, er := range gltfDoc.Extensions {
		fmt.Printf("Extension %v is defined\n", er)
	}
	fmt.Println("ExtensionsRequired=", len(gltfDoc.ExtensionsRequired))
	for _, er := range gltfDoc.ExtensionsRequired {
		fmt.Printf("Extension %s is required\n", er)
	}
	fmt.Println("ExtensionsUsed=", len(gltfDoc.ExtensionsUsed))
	for _, er := range gltfDoc.ExtensionsUsed {
		fmt.Printf("Extension %s is used\n", er)
	}
	fmt.Println("Materials=", len(gltfDoc.Materials))
	for _, a := range gltfDoc.Materials {
		fmt.Println("Materials", a.Name)
	}
	fmt.Println("Meshes=", len(gltfDoc.Meshes))
	for _, a := range gltfDoc.Meshes {
		fmt.Println("Meshes", a.Name)
	}
	fmt.Println("Samplers=", len(gltfDoc.Samplers))
	fmt.Println("Skins=", len(gltfDoc.Skins))
	for _, a := range gltfDoc.Skins {
		fmt.Println("Skins", a.Name)
	}
	fmt.Println("Textures=", len(gltfDoc.Textures))
	for _, a := range gltfDoc.Textures {
		fmt.Println("Textures", a.Name)
	}

	// doc, _ := gltf.Open(absFilePath)
	// pd, _ := draco.UnmarshalMesh(doc, doc.BufferViews[0])
	// p := doc.Meshes[0].Primitives[0]
	// fmt.Println(pd.ReadIndices(nil))
	// fmt.Println(pd.ReadAttr(p, "POSITION", nil))
	// fmt.Println(pd.ReadAttr(p, "NORMAL", nil))

	scene, err := gltfDoc.LoadScene(*gltfDoc.Scene)
	fmt.Println("Load Scene=", scene, "Error=", err)
	// return
	// if err != nil {
	// 	panic(err)
	// }
	// Create application and scene
	a := app.App()
	// scene := core.NewNode()

	// Set the scene to be managed by the gui manager
	gui.Manager().Set(scene)

	// Create perspective camera
	cam := camera.New(1)
	cam.SetPosition(0, 0, 3)
	scene.GetNode().Add(cam)

	// Set up orbit control for the camera
	camera.NewOrbitControl(cam)

	// Set up callback to update viewport and camera aspect ratio when the window is resized
	onResize := func(evname string, ev interface{}) {
		// Get framebuffer size and update viewport accordingly
		width, height := a.GetSize()
		a.Gls().Viewport(0, 0, int32(width), int32(height))
		// Update the camera's aspect ratio
		cam.SetAspect(float32(width) / float32(height))
	}
	a.Subscribe(window.OnWindowSize, onResize)
	onResize("", nil)

	// Create a blue torus and add it to the scene
	// geom := geometry.NewTorus(1, .4, 12, 32, math32.Pi*2)
	// mat := material.NewStandard(math32.NewColor("DarkBlue"))
	// mesh := graphic.NewMesh(geom, mat)
	// scene.Add(mesh)

	// Create and add a button to the scene
	// for idx, anim := range anims {
	// 	btn := gui.NewButton(anim.Name())
	// 	btn.SetPosition(100, float32(40*(1+idx)))
	// 	btn.SetSize(40, 40)
	// 	btn.Subscribe(gui.OnClick, func(name string, ev interface{}) {
	// 		anim.Reset()
	// 	})
	// 	scene.GetNode().Add(btn)
	// }

	// Create and add lights to the scene
	scene.GetNode().Add(light.NewAmbient(&math32.Color{1.0, 1.0, 1.0}, 0.8))
	pointLight := light.NewPoint(&math32.Color{1, 1, 1}, 5.0)
	pointLight.SetPosition(1, 0, 2)
	scene.GetNode().Add(pointLight)

	// Create and add an axis helper to the scene
	scene.GetNode().Add(helper.NewAxes(0.5))

	// Set background color to gray
	a.Gls().ClearColor(0.5, 0.5, 0.5, 1.0)

	// Run the application
	a.Run(func(renderer *renderer.Renderer, deltaTime time.Duration) {
		a.Gls().Clear(gls.DEPTH_BUFFER_BIT | gls.STENCIL_BUFFER_BIT | gls.COLOR_BUFFER_BIT)
		renderer.Render(scene, cam)
	})
}
