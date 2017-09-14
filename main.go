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
type Depth struct {
	format   vk.Format
	image    vk.Image
	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

type Application struct {
	as.BaseVulkanApp
	windowHandle uintptr
	debugEnabled bool
	height uint32
	width uint32
	depth *Depth

	pipelineLayout vk.PipelineLayout
	// descLayout     vk.DescriptorSetLayout
	pipelineCache  vk.PipelineCache
	renderPass     vk.RenderPass
	pipeline       vk.Pipeline
}

func (a *Application) VulkanContextPrepare() error {
	dim := a.Context().SwapchainDimensions()
	a.height = dim.Height
	a.width = dim.Width

	a.prepareDepth()
	// s.prepareTextures()
	// s.prepareCubeDataBuffers()
	// s.prepareDescriptorLayout()
	// s.prepareRenderPass()
	// s.preparePipeline()
	// s.prepareDescriptorPool()
	// s.prepareDescriptorSet()
	// s.prepareFramebuffers()

	swapchainImageResources := a.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		a.drawBuildCommandBuffer(res, res.CommandBuffer())
	}
	return nil
}

func (a *Application) prepareDepth() {
	dev := a.Context().Device()
	depthFormat := vk.FormatD16Unorm
	a.depth = &Depth{
		format: depthFormat,
	}
	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    depthFormat,
		Extent: vk.Extent3D{
			Width:  a.width,
			Height: a.height,
			Depth:  1,
		},
		MipLevels:   1,
		ArrayLayers: 1,
		Samples:     vk.SampleCount1Bit,
		Tiling:      vk.ImageTilingOptimal,
		Usage:       vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
	}, nil, &a.depth.image)
	orPanic(as.NewError(ret))

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(dev, a.depth.image, &memReqs)
	memReqs.Deref()

	memProps := a.Context().Platform().MemoryProperties()
	memTypeIndex, _ := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), vk.MemoryPropertyDeviceLocalBit)
	a.depth.memAlloc = &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}

	var mem vk.DeviceMemory
	ret = vk.AllocateMemory(dev, a.depth.memAlloc, nil, &mem)
	orPanic(as.NewError(ret))
	a.depth.mem = mem

	ret = vk.BindImageMemory(dev, a.depth.image, a.depth.mem, 0)
	orPanic(as.NewError(ret))

	var view vk.ImageView
	ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
		SType:  vk.StructureTypeImageViewCreateInfo,
		Format: depthFormat,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectDepthBit),
			LevelCount: 1,
			LayerCount: 1,
		},
		ViewType: vk.ImageViewType2d,
		Image:    a.depth.image,
	}, nil, &view)
	orPanic(as.NewError(ret))
	a.depth.view = view
}

func (a *Application) drawBuildCommandBuffer(res *as.SwapchainImageResources, cmd vk.CommandBuffer) {
	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	})
	orPanic(as.NewError(ret))

	clearValues := make([]vk.ClearValue, 2)
	clearValues[1].SetDepthStencil(1, 0)
	clearValues[0].SetColor([]float32{
		0.2, 0.2, 0.2, 0.2,
	})
	vk.CmdBeginRenderPass(cmd, &vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  a.renderPass,
		Framebuffer: res.Framebuffer(),
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{
				X: 0, Y: 0,
			},
			Extent: vk.Extent2D{
				Width:  a.width,
				Height: a.height,
			},
		},
		ClearValueCount: 2,
		PClearValues:    clearValues,
	}, vk.SubpassContentsInline)

	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, a.pipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, a.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)
	vk.CmdSetViewport(cmd, 0, 1, []vk.Viewport{{
		Width:    float32(a.width),
		Height:   float32(a.height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}})
	vk.CmdSetScissor(cmd, 0, 1, []vk.Rect2D{{
		Offset: vk.Offset2D{
			X: 0, Y: 0,
		},
		Extent: vk.Extent2D{
			Width:  a.width,
			Height: a.height,
		},
	}})

	vk.CmdDraw(cmd, 12*3, 1, 0, 0)
	// Note that ending the renderpass changes the image's layout from
	// vk.ImageLayoutColorAttachmentOptimal to vk.ImageLayoutPresentSrc
	vk.CmdEndRenderPass(cmd)

	graphicsQueueIndex := a.Context().Platform().GraphicsQueueFamilyIndex()
	presentQueueIndex := a.Context().Platform().PresentQueueFamilyIndex()
	if graphicsQueueIndex != presentQueueIndex {
		// Separate Present Queue Case
		//
		// We have to transfer ownership from the graphics queue family to the
		// present queue family to be able to present.  Note that we don't have
		// to transfer from present queue family back to graphics queue family at
		// the start of the next frame because we don't care about the image's
		// contents at that point.
		vk.CmdPipelineBarrier(cmd,
			vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit),
			0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{{
				SType:               vk.StructureTypeImageMemoryBarrier,
				SrcAccessMask:       0,
				DstAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
				OldLayout:           vk.ImageLayoutPresentSrc,
				NewLayout:           vk.ImageLayoutPresentSrc,
				SrcQueueFamilyIndex: graphicsQueueIndex,
				DstQueueFamilyIndex: presentQueueIndex,
				SubresourceRange: vk.ImageSubresourceRange{
					AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
					LayerCount: 1,
					LevelCount: 1,
				},
				Image: res.Image(),
			}})
	}
	ret = vk.EndCommandBuffer(cmd)
	orPanic(as.NewError(ret))
}

func (a *Application) VulkanSurface(instance vk.Instance) (surface vk.Surface) {
	ret := vk.CreateWindowSurface(instance, a.windowHandle, nil, &surface)
	if err := vk.Error(ret); err != nil {
		log.Panicln("vulkan error:", err)
		return vk.NullSurface
	}
	return surface
}

func (a *Application) VulkanAppName() string {
	return "test"
}

func (a *Application) VulkanDebug() bool {
	return false
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