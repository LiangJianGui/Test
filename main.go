package main

import (
	"log"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/vulkan-go/glfw/v3.3/glfw"
	"github.com/xlab/closer"

	"time"
)
///////////////////////////////////////////////////////////////////////////////
type Application struct {
	as.BaseVulkanApp
	windowHandle uintptr
	debugEnabled bool
}

func (a *Application) VulkanSurface(instance vk.Instance) (surface vk.Surface) {
	ret := vk.CreateWindowSurface(instance, a.windowHandle, nil, &surface)
	if err := vk.Error(ret); err != nil {
		log.Panicln("vulkan error:", err)
		return vk.NullSurface
	}
	return surface
}

func (a *Application) VulkanDebug() bool {
	return false
}

func (a *Application) VulkanAppName() string {
	return "test"
}

func (a *Application) VulkanSwapchainDimensions() *as.SwapchainDimensions {
	return &as.SwapchainDimensions{
		Width: 500, Height: 500, Format: vk.FormatB8g8r8a8Unorm,
	}
}

func (a *Application) VulkanInstanceExtensions() []string {
	extensions := vk.GetRequiredInstanceExtensions()
	if a.debugEnabled {
		extensions = append(extensions, "VK_EXT_debug_report")
	}
	return extensions
}

func (a *Application) Destroy()  {
	
}

func NewApplication(debugEnabled bool) *Application {
	return &Application{
		debugEnabled: debugEnabled,
	}
}
////////////////////////////////////////////////////////////////////////////


func main() {
	glfw.Init()
	vk.Init()
	defer closer.Close()
	app := NewApplication(true)
	reqDim := app.VulkanSwapchainDimensions()
	glfw.WindowHint(glfw.ClientAPI,glfw.NoAPI)
	window, _ := glfw.CreateWindow(int(reqDim.Width),int(reqDim.Height),app.VulkanAppName(),nil,nil)
	app.windowHandle = window.GLFWWindow()

	platform, err := as.NewPlatform(app)
	orPanic(err)

	doneC := make(chan struct{},2)
	exitC := make(chan struct{},2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})
	fpsDelay := time.Second /60
	fpsTicker := time.NewTicker(fpsDelay)

	for {
		select {
		case <-exitC:
			app.Destroy()
			platform.Destroy()
			window.Destroy()
			glfw.Terminate()
			fpsTicker.Stop()
			doneC <- struct{}{}
			return
		case <-fpsTicker.C:
			if window.ShouldClose() {
				exitC <- struct{}{}
				continue
			}
			glfw.PollEvents()
		}
	}
}

func orPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(err)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	}
}