package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

type Model struct {
	service                  application.ContainerService
	memoryMode               config.MemoryMode
	snapshot                 domain.Snapshot
	err                      error
	loading                  bool
	refreshing               bool
	generation               uint64
	width                    int
	height                   int
	active                   int
	selectedID               string
	stacks                   []domain.Stack
	stacksLoading            bool
	stacksLoaded             bool
	stacksErr                error
	stackDiagnostics         []string
	stacksGen                uint64
	selectedStackName        string
	selectedStackContainerID string
	expandedStackName        string
	stackEditing             bool
	selectedStacks           map[string]struct{}
	stackContainerEditing    bool
	selectedStackContainers  map[string]struct{}
	images                   []domain.Image
	imagesLoading            bool
	imagesLoaded             bool
	imagesErr                error
	imagesGen                uint64
	selectedImageID          string
	imageDetails             domain.ImageDetails
	imageDetailErr           error
	imageDetailLoading       bool
	imageDetailOpen          bool
	imageEditing             bool
	selectedImages           map[string]struct{}
	networks                 []domain.Network
	networksLoading          bool
	networksLoaded           bool
	networksErr              error
	networksGen              uint64
	selectedNetworkID        string
	networkDetails           domain.NetworkDetails
	networkDetailErr         error
	networkDetailLoading     bool
	networkDetailOpen        bool
	networkEditing           bool
	selectedNetworks         map[string]struct{}
	volumes                  []domain.Volume
	volumesLoading           bool
	volumesLoaded            bool
	volumesErr               error
	volumesGen               uint64
	selectedVolumeName       string
	volumeDetails            domain.VolumeDetails
	volumeDetailErr          error
	volumeDetailLoading      bool
	volumeDetailOpen         bool
	volumeEditing            bool
	selectedVolumes          map[string]struct{}
	showHelp                 bool
	sortMode                 application.SortMode
	editing                  bool
	selected                 map[string]struct{}
	action                   actionState
	panel                    panelMode
	details                  domain.ContainerDetails
	detailErr                error
	detailLoading            bool
	logLines                 []string
	logErr                   error
	logOffset                int
	logActive                bool
	logCancel                context.CancelFunc
	logGen                   uint64
	stackLogs                bool
	logTitle                 string
	shellActive              bool
	shellErr                 error
	keys                     keyMap
	now                      func() time.Time
	ctx                      context.Context
	cancel                   context.CancelFunc
}

type keyMap struct {
	quit      key.Binding
	next      key.Binding
	prev      key.Binding
	retry     key.Binding
	up        key.Binding
	down      key.Binding
	help      key.Binding
	back      key.Binding
	sort      key.Binding
	edit      key.Binding
	selectRow key.Binding
	confirm   key.Binding
	logs      key.Binding
	shell     key.Binding
}

type panelMode int

const (
	panelContainers panelMode = iota
	panelDetails
	panelLogs
)

type actionStage int

const (
	actionNone actionStage = iota
	actionMenu
	actionConfirm
	actionResult
)

type actionResource int

const (
	actionContainers actionResource = iota
	actionImages
	actionNetworks
	actionVolumes
	actionStacks
	actionStackContainers
)

type actionTarget struct {
	ID          string
	Name        string
	State       string
	Unavailable string
}

type actionState struct {
	stage    actionStage
	resource actionResource
	index    int
	targets  []actionTarget
	input    string
	running  bool
	results  []application.ActionResult
	choices  []application.Action
}

type loadedMsg struct {
	snapshot   domain.Snapshot
	err        error
	generation uint64
}

type refreshMsg time.Time

type actionFinishedMsg struct {
	results []application.ActionResult
}

type detailsLoadedMsg struct {
	details domain.ContainerDetails
	err     error
}

type logsOpenedMsg struct {
	stream ports.LogStream
	err    error
	gen    uint64
}

type logLineMsg struct {
	line   string
	stream ports.LogStream
	gen    uint64
}

type logErrorMsg struct {
	err error
	gen uint64
}

type logsClosedMsg struct{ gen uint64 }

type shellFinishedMsg struct{ err error }

var makeShellCommand = func(containerID string) *exec.Cmd {
	return exec.Command("docker", "exec", "-it", containerID, "/bin/sh", "-l")
}

type imagesLoadedMsg struct {
	images     []domain.Image
	err        error
	generation uint64
}

type stacksLoadedMsg struct {
	stacks     []domain.Stack
	err        error
	generation uint64
}

type resourcesLoadedMsg struct {
	resources ports.ResourceLoad
	err       error
}

type imageDetailsLoadedMsg struct {
	details domain.ImageDetails
	err     error
}

type networksLoadedMsg struct {
	networks   []domain.Network
	err        error
	generation uint64
}
type networkDetailsLoadedMsg struct {
	details domain.NetworkDetails
	err     error
}
type volumesLoadedMsg struct {
	volumes    []domain.Volume
	err        error
	generation uint64
}
type volumeDetailsLoadedMsg struct {
	details domain.VolumeDetails
	err     error
}

const refreshInterval = 2 * time.Second

func NewModel(service application.ContainerService, memoryMode config.MemoryMode, stackDiagnostics ...string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		service:                 service,
		memoryMode:              memoryMode,
		loading:                 true,
		stacksLoading:           true,
		stackDiagnostics:        append([]string(nil), stackDiagnostics...),
		imagesLoading:           true,
		networksLoading:         true,
		volumesLoading:          true,
		generation:              1,
		sortMode:                application.SortState,
		selected:                make(map[string]struct{}),
		selectedImages:          make(map[string]struct{}),
		selectedNetworks:        make(map[string]struct{}),
		selectedVolumes:         make(map[string]struct{}),
		selectedStacks:          make(map[string]struct{}),
		selectedStackContainers: make(map[string]struct{}),
		keys: keyMap{
			quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			next:      key.NewBinding(key.WithKeys("right"), key.WithHelp("right", "next view")),
			prev:      key.NewBinding(key.WithKeys("left"), key.WithHelp("left", "previous view")),
			retry:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
			up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
			down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
			help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			sort:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "sort")),
			edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
			selectRow: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
			confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "actions")),
			logs:      key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "logs")),
			shell:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shell")),
		},
		now:    time.Now,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(m.generation), m.loadResources())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.quit) {
			m.closeLogs()
			m.cancel()
			return m, tea.Quit
		}
		if m.action.running {
			return m, nil
		}
		if m.action.stage == actionResult {
			if key.Matches(msg, m.keys.back) || key.Matches(msg, m.keys.confirm) {
				m.action = actionState{}
			}
			return m, nil
		}
		if m.action.stage == actionConfirm {
			return m.updateConfirmation(msg)
		}
		if m.action.stage == actionMenu {
			return m.updateActionMenu(msg)
		}
		if m.panel == panelLogs {
			return m.updateLogs(msg)
		}
		if m.active == 1 {
			return m.updateStacks(msg)
		}
		if m.active == 2 {
			return m.updateImages(msg)
		}
		if m.active == 3 {
			return m.updateNetworks(msg)
		}
		if m.active == 4 {
			return m.updateVolumes(msg)
		}
		if m.panel == panelLogs {
			return m.updateLogs(msg)
		}
		if m.panel == panelDetails {
			if key.Matches(msg, m.keys.back) {
				m.panel = panelContainers
				return m, nil
			}
			if key.Matches(msg, m.keys.logs) {
				return m.startLogs()
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.back):
			if m.editing {
				m.clearSelection()
				m.editing = false
			} else {
				m.showHelp = false
			}
			return m, nil
		case key.Matches(msg, m.keys.edit):
			if m.editing {
				m.clearSelection()
				m.editing = false
			} else {
				m.editing = true
			}
			return m, nil
		case key.Matches(msg, m.keys.selectRow):
			if m.editing {
				m.toggleSelection(m.selectedID)
			}
			return m, nil
		case key.Matches(msg, m.keys.confirm):
			if m.editing && len(m.selected) > 0 {
				m.action = actionState{stage: actionMenu, targets: m.selectedTargets()}
				return m, nil
			}
			return m.openDetails()
		case key.Matches(msg, m.keys.logs):
			return m.startLogs()
		case key.Matches(msg, m.keys.shell):
			if !m.editing {
				return m.startShellFor(m.selectedID)
			}
			return m, nil
		case key.Matches(msg, m.keys.sort):
			m.sortMode = application.NextSortMode(m.sortMode)
			m.snapshot = m.service.Sort(m.snapshot, m.sortMode)
			return m, nil
		case key.Matches(msg, m.keys.next):
			m.active = (m.active + 1) % 5
			return m.startActiveLoad()
		case key.Matches(msg, m.keys.prev):
			if m.active == 0 {
				m.active = 4
			} else {
				m.active--
			}
			return m.startActiveLoad()
		case key.Matches(msg, m.keys.retry):
			m.generation++
			m.loading = len(m.snapshot.Containers) == 0
			m.refreshing = !m.loading
			m.err = nil
			return m, m.load(m.generation)
		case key.Matches(msg, m.keys.up):
			m = m.moveSelection(-1)
			return m, nil
		case key.Matches(msg, m.keys.down):
			m = m.moveSelection(1)
			return m, nil
		}
	case loadedMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.loading = false
		m.refreshing = false
		if msg.err == nil {
			m.snapshot = m.service.Sort(msg.snapshot, m.sortMode)
			m.stacks = m.service.RebuildStacks(m.snapshot)
			m.syncStackSelection()
		}
		m.err = msg.err
		m = m.syncSelection()
		return m, scheduleRefresh()
	case refreshMsg:
		if m.loading || m.refreshing {
			return m, nil
		}
		m.generation++
		m.refreshing = true
		return m, m.load(m.generation)
	case actionFinishedMsg:
		m.action.running = false
		m.action.stage = actionResult
		m.action.results = msg.results
		if m.action.resource == actionImages {
			m.imageEditing = false
			m.clearImageSelection()
			m.imagesGen++
			m.imagesLoaded = false
			m.imagesLoading = true
			m.imagesErr = nil
			return m, m.loadImages(m.imagesGen)
		}
		if m.action.resource == actionNetworks {
			m.networkEditing = false
			m.clearNetworkSelection()
			m.networksGen++
			m.networksLoaded = false
			m.networksLoading = true
			m.networksErr = nil
			return m, m.loadNetworks(m.networksGen)
		}
		if m.action.resource == actionVolumes {
			m.volumeEditing = false
			m.clearVolumeSelection()
			m.volumesGen++
			m.volumesLoaded = false
			m.volumesLoading = true
			m.volumesErr = nil
			return m, m.loadVolumes(m.volumesGen)
		}
		if m.action.resource == actionStacks {
			m.stackEditing = false
			m.clearStackSelection()
			m.generation++
			m.refreshing = true
			m.stacksGen++
			m.stacksLoaded, m.stacksLoading, m.stacksErr = false, true, nil
			return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen))
		}
		if m.action.resource == actionStackContainers {
			m.stackContainerEditing = false
			m.clearStackContainerSelection()
			m.generation++
			m.refreshing = true
			m.stacksGen++
			m.stacksLoaded, m.stacksLoading, m.stacksErr = false, true, nil
			return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen))
		}
		m.editing = false
		m.clearSelection()
		m.generation++
		return m, m.load(m.generation)
	case resourcesLoadedMsg:
		// A manual refresh started during initialization owns that resource's state.
		if m.stacksGen == 0 {
			stacks := application.EnrichStacks(msg.resources.Stacks, m.snapshot)
			if !m.loading && m.err == nil {
				stacks = m.service.RebuildStacks(m.snapshot)
			}
			m.stacksLoading, m.stacksLoaded, m.stacks, m.stacksErr = false, true, stacks, msg.err
			if msg.err == nil {
				m.stacksErr = msg.resources.StacksErr
			}
			m.syncStackSelection()
		}
		if m.imagesGen == 0 {
			m.imagesLoading, m.imagesLoaded, m.images, m.imagesErr = false, true, msg.resources.Images, msg.err
			if msg.err == nil {
				m.imagesErr = msg.resources.ImagesErr
			}
			m.syncImageSelection()
		}
		if m.networksGen == 0 {
			m.networksLoading, m.networksLoaded, m.networks, m.networksErr = false, true, msg.resources.Networks, msg.err
			if msg.err == nil {
				m.networksErr = msg.resources.NetworksErr
			}
			m.syncNetworkSelection()
		}
		if m.volumesGen == 0 {
			m.volumesLoading, m.volumesLoaded, m.volumes, m.volumesErr = false, true, msg.resources.Volumes, msg.err
			if msg.err == nil {
				m.volumesErr = msg.resources.VolumesErr
			}
			m.syncVolumeSelection()
		}
		return m, nil
	case imagesLoadedMsg:
		if msg.generation != m.imagesGen {
			return m, nil
		}
		m.imagesLoading = false
		m.imagesLoaded = true
		m.images = msg.images
		m.imagesErr = msg.err
		m.syncImageSelection()
		return m, nil
	case stacksLoadedMsg:
		if msg.generation != m.stacksGen {
			return m, nil
		}
		m.stacksLoading, m.stacksLoaded, m.stacks, m.stacksErr = false, true, application.EnrichStacks(msg.stacks, m.snapshot), msg.err
		m.syncStackSelection()
		return m, nil
	case imageDetailsLoadedMsg:
		if !m.imageDetailOpen {
			return m, nil
		}
		m.imageDetails = msg.details
		m.imageDetailErr = msg.err
		m.imageDetailLoading = false
		return m, nil
	case networksLoadedMsg:
		if msg.generation != m.networksGen {
			return m, nil
		}
		m.networksLoading, m.networksLoaded, m.networks, m.networksErr = false, true, msg.networks, msg.err
		m.syncNetworkSelection()
		return m, nil
	case networkDetailsLoadedMsg:
		if !m.networkDetailOpen {
			return m, nil
		}
		m.networkDetails, m.networkDetailErr, m.networkDetailLoading = msg.details, msg.err, false
		return m, nil
	case volumesLoadedMsg:
		if msg.generation != m.volumesGen {
			return m, nil
		}
		m.volumesLoading, m.volumesLoaded, m.volumes, m.volumesErr = false, true, msg.volumes, msg.err
		m.syncVolumeSelection()
		return m, nil
	case volumeDetailsLoadedMsg:
		if !m.volumeDetailOpen {
			return m, nil
		}
		m.volumeDetails, m.volumeDetailErr, m.volumeDetailLoading = msg.details, msg.err, false
		return m, nil
	case detailsLoadedMsg:
		if m.panel != panelDetails {
			return m, nil
		}
		m.details = msg.details
		m.detailErr = msg.err
		m.detailLoading = false
		return m, nil
	case logsOpenedMsg:
		if msg.gen != m.logGen || m.panel != panelLogs {
			return m, nil
		}
		if msg.err != nil {
			m.logActive = false
			m.logErr = msg.err
			return m, nil
		}
		m.logActive = true
		return m, m.waitLog(msg.stream, msg.gen)
	case logLineMsg:
		if msg.gen != m.logGen || m.panel != panelLogs {
			return m, nil
		}
		m.logLines = append(m.logLines, msg.line)
		if len(m.logLines) > 1000 {
			m.logLines = m.logLines[len(m.logLines)-1000:]
		}
		if m.logOffset > len(m.logLines)-1 {
			m.logOffset = len(m.logLines) - 1
		}
		return m, m.waitLog(msg.stream, msg.gen)
	case logErrorMsg:
		if msg.gen == m.logGen && m.panel == panelLogs {
			m.logActive = false
			m.logErr = msg.err
		}
		return m, nil
	case logsClosedMsg:
		if msg.gen == m.logGen && m.panel == panelLogs {
			m.logActive = false
		}
		return m, nil
	case shellFinishedMsg:
		m.shellActive = false
		m.shellErr = msg.err
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	layout := sharedui.ResolveLayout(m.width, m.height)
	shell := sharedui.NewShell(sharedui.ShellOptions{
		Title:      "dtop",
		Subtitle:   m.headerSummary(),
		ActiveView: m.active,
		Footer:     m.footer(layout),
		Views: []sharedui.View{
			m.containersView(layout),
			m.stacksView(layout),
			m.imagesView(layout),
			m.networksView(layout),
			m.volumesView(layout),
		},
	})

	if m.width > 0 || m.height > 0 {
		updated, _ := shell.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		shell = updated
	}

	return shell.View()
}

func (m Model) imagesView(layout sharedui.Layout) sharedui.View {
	if m.action.resource == actionImages && m.action.stage != actionNone {
		return m.imagesActionView()
	}
	if m.imageDetailOpen {
		return m.imageDetailsView()
	}
	if m.imagesLoading || !m.imagesLoaded {
		return sharedui.View{Title: "Images", Status: sharedui.StatusLoading, Summary: "Loading Docker images..."}
	}
	if m.imagesErr != nil {
		return sharedui.View{
			Title: "Images", Status: sharedui.StatusError, Summary: m.imagesErr.Error(),
			Sections: []sharedui.Section{{Title: "Next", Body: "Press r to retry."}},
		}
	}
	if len(m.images) == 0 {
		return sharedui.View{Title: "Images", Status: sharedui.StatusEmpty, Summary: "No Docker images were found."}
	}
	return sharedui.View{
		Title: "Images", Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{{Body: renderImages(m.images, m.selectedImageID, m.selectedImages, m.imageEditing, layout, m.now())}},
	}
}

func (m Model) imageDetailsView() sharedui.View {
	if m.imageDetailLoading {
		return sharedui.View{Title: "Image details", Status: sharedui.StatusLoading, Summary: "Loading image details..."}
	}
	if m.imageDetailErr != nil {
		return sharedui.View{
			Title: "Image details", Status: sharedui.StatusError, Summary: m.imageDetailErr.Error(),
			Sections: []sharedui.Section{{Title: "Controls", Body: "esc back"}},
		}
	}
	details := m.imageDetails
	tags := "<untagged>"
	if len(details.Tags) > 0 {
		tags = strings.Join(details.Tags, "\n")
	}
	digests := "-"
	if len(details.Digests) > 0 {
		digests = strings.Join(details.Digests, "\n")
	}
	return sharedui.View{
		Title: "Image details", Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{
			{Title: shortContainerID(details.ID), Body: strings.Join([]string{"Size: " + formatBytes(details.Size), "Created: " + formatImageAge(details.Created, m.now()), "Platform: " + details.OS + "/" + details.Architecture}, "\n")},
			{Title: "Tags", Body: tags},
			{Title: "Digests", Body: digests},
			{Title: "Controls", Body: "esc back"},
		},
	}
}

func (m Model) networksView(layout sharedui.Layout) sharedui.View {
	if m.networkDetailOpen {
		return m.networkDetailsView()
	}
	if m.networksLoading || !m.networksLoaded {
		return sharedui.View{Title: "Networks", Status: sharedui.StatusLoading, Summary: "Loading Docker networks..."}
	}
	if m.networksErr != nil {
		return sharedui.View{Title: "Networks", Status: sharedui.StatusError, Summary: m.networksErr.Error(), Sections: []sharedui.Section{{Title: "Next", Body: "Press r to retry."}}}
	}
	if len(m.networks) == 0 {
		return sharedui.View{Title: "Networks", Status: sharedui.StatusEmpty, Summary: "No Docker networks were found."}
	}
	if m.action.resource == actionNetworks && m.action.stage != actionNone {
		return m.actionView()
	}
	return sharedui.View{Title: "Networks", Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Body: renderNetworks(m.networks, m.selectedNetworkID, m.selectedNetworks, m.networkEditing, layout, m.now())}}}
}

func (m Model) networkDetailsView() sharedui.View {
	if m.networkDetailLoading {
		return sharedui.View{Title: "Network details", Status: sharedui.StatusLoading, Summary: "Loading network details..."}
	}
	if m.networkDetailErr != nil {
		return sharedui.View{Title: "Network details", Status: sharedui.StatusError, Summary: m.networkDetailErr.Error(), Sections: []sharedui.Section{{Title: "Controls", Body: "esc back"}}}
	}
	details := m.networkDetails
	containers := "-"
	if len(details.Containers) > 0 {
		containers = strings.Join(details.Containers, "\n")
	}
	return sharedui.View{Title: "Network details", Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Title: details.Name, Body: strings.Join([]string{"ID: " + shortContainerID(details.ID), "Driver: " + details.Driver, "Scope: " + details.Scope, "Created: " + formatImageAge(details.Created, m.now()), "Internal: " + yesNo(details.Internal), "Attachable: " + yesNo(details.Attachable)}, "\n")}, {Title: "Containers", Body: containers}, {Title: "Controls", Body: "esc back"}}}
}

func (m Model) volumesView(layout sharedui.Layout) sharedui.View {
	if m.volumeDetailOpen {
		return m.volumeDetailsView()
	}
	if m.volumesLoading || !m.volumesLoaded {
		return sharedui.View{Title: "Volumes", Status: sharedui.StatusLoading, Summary: "Loading Docker volumes..."}
	}
	if m.volumesErr != nil {
		return sharedui.View{Title: "Volumes", Status: sharedui.StatusError, Summary: m.volumesErr.Error(), Sections: []sharedui.Section{{Title: "Next", Body: "Press r to retry."}}}
	}
	if len(m.volumes) == 0 {
		return sharedui.View{Title: "Volumes", Status: sharedui.StatusEmpty, Summary: "No Docker volumes were found."}
	}
	if m.action.resource == actionVolumes && m.action.stage != actionNone {
		return m.actionView()
	}
	return sharedui.View{Title: "Volumes", Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Body: renderVolumes(m.volumes, m.selectedVolumeName, m.selectedVolumes, m.volumeEditing, layout, m.now())}}}
}

func (m Model) volumeDetailsView() sharedui.View {
	if m.volumeDetailLoading {
		return sharedui.View{Title: "Volume details", Status: sharedui.StatusLoading, Summary: "Loading volume details..."}
	}
	if m.volumeDetailErr != nil {
		return sharedui.View{Title: "Volume details", Status: sharedui.StatusError, Summary: m.volumeDetailErr.Error(), Sections: []sharedui.Section{{Title: "Controls", Body: "esc back"}}}
	}
	details := m.volumeDetails
	options := "-"
	if len(details.Options) > 0 {
		lines := make([]string, 0, len(details.Options))
		for key, value := range details.Options {
			lines = append(lines, key+"="+value)
		}
		sort.Strings(lines)
		options = strings.Join(lines, "\n")
	}
	return sharedui.View{Title: "Volume details", Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Title: details.Name, Body: strings.Join([]string{"Driver: " + details.Driver, "Scope: " + details.Scope, "Mountpoint: " + details.Mountpoint, "Created: " + formatImageAge(details.Created, m.now())}, "\n")}, {Title: "Options", Body: options}, {Title: "Controls", Body: "esc back"}}}
}

func (m Model) load(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()

		snapshot, err := m.service.Load(ctx)
		return loadedMsg{snapshot: snapshot, err: err, generation: generation}
	}
}

func (m Model) loadResources() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()

		resources, err := m.service.LoadResources(ctx)
		return resourcesLoadedMsg{resources: resources, err: err}
	}
}

func (m Model) startImageLoad() (tea.Model, tea.Cmd) {
	if m.active != 2 || m.imagesLoaded || m.imagesLoading {
		return m, nil
	}
	m.imagesGen++
	m.imagesLoading = true
	return m, m.loadImages(m.imagesGen)
}

func (m Model) startActiveLoad() (tea.Model, tea.Cmd) {
	switch m.active {
	case 1:
		return m.startStackLoad()
	case 2:
		return m.startImageLoad()
	case 3:
		return m.startNetworkLoad()
	case 4:
		return m.startVolumeLoad()
	default:
		return m, nil
	}
}

func (m Model) startStackLoad() (tea.Model, tea.Cmd) {
	if m.active != 1 || m.stacksLoaded || m.stacksLoading {
		return m, nil
	}
	m.stacksGen++
	m.stacksLoading = true
	return m, m.loadStacks(m.stacksGen)
}

func (m Model) loadStacks(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		stacks, err := m.service.Stacks(ctx)
		return stacksLoadedMsg{stacks: stacks, err: err, generation: generation}
	}
}

func (m Model) updateStacks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.next):
		m.active = 2
		return m.startActiveLoad()
	case key.Matches(msg, m.keys.prev):
		m.active = 0
		return m, nil
	case key.Matches(msg, m.keys.back):
		if m.selectedStackContainerID != "" {
			m.selectedStackContainerID = ""
			m.stackContainerEditing = false
			m.clearStackContainerSelection()
			m.expandedStackName = ""
		} else if m.stackEditing {
			m.stackEditing = false
			m.clearStackSelection()
		} else {
			m.expandedStackName = ""
		}
	case key.Matches(msg, m.keys.edit):
		if m.selectedStackContainerID != "" {
			m.stackContainerEditing = !m.stackContainerEditing
			if !m.stackContainerEditing {
				m.clearStackContainerSelection()
			}
		} else {
			m.stackEditing = !m.stackEditing
			if !m.stackEditing {
				m.clearStackSelection()
			}
		}
	case key.Matches(msg, m.keys.selectRow):
		if m.stackContainerEditing && m.selectedStackContainerID != "" {
			m.toggleStackContainerSelection(m.selectedStackContainerID)
		} else if m.stackEditing {
			m.toggleStackSelection(m.selectedStackName)
		}
	case key.Matches(msg, m.keys.retry):
		m.stacksGen++
		m.stacksLoading, m.stacksErr = true, nil
		return m, m.loadStacks(m.stacksGen)
	case key.Matches(msg, m.keys.up):
		m = m.moveStackSelection(-1)
	case key.Matches(msg, m.keys.down):
		m = m.moveStackSelection(1)
	case key.Matches(msg, m.keys.logs):
		if m.selectedStackContainerID != "" {
			return m.startLogsFor(m.selectedStackContainerID)
		}
		if stack := m.selectedStack(); stack != nil && stack.DownUnavailableReason() == "" {
			return m.startStackLogs(*stack)
		}
	case key.Matches(msg, m.keys.shell):
		if m.selectedStackContainerID == "" || m.expandedStackName != m.selectedStackName {
			return m, nil
		}
		return m.startShellFor(m.selectedStackContainerID)
	case key.Matches(msg, m.keys.confirm):
		if m.selectedStackContainerID != "" {
			targets := m.selectedStackContainerTargets()
			if !m.stackContainerEditing {
				targets = m.selectedStackContainerTargetsForIDs([]string{m.selectedStackContainerID})
			}
			if len(targets) > 0 {
				m.action = actionState{stage: actionMenu, resource: actionStackContainers, targets: targets}
			}
			return m, nil
		}
		if m.stackEditing && len(m.selectedStacks) > 0 {
			targets := m.selectedStackTargets()
			m.action = actionState{stage: actionMenu, resource: actionStacks, targets: targets, choices: stackActionChoices(targets)}
			return m, nil
		}
		if m.selectedStackName != "" {
			if m.expandedStackName == m.selectedStackName {
				m.expandedStackName = ""
			} else {
				m.expandedStackName = m.selectedStackName
				if stack := m.selectedStack(); stack != nil && len(stack.ContainerItems) > 0 {
					m.selectedStackContainerID = stack.ContainerItems[0].ID
				}
			}
		}
	}
	return m, nil
}

func (m Model) loadImages(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		images, err := m.service.Images(ctx)
		return imagesLoadedMsg{images: images, err: err, generation: generation}
	}
}

func (m Model) updateImages(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.next), key.Matches(msg, m.keys.prev):
		if key.Matches(msg, m.keys.next) {
			m.active = 3
		} else {
			m.active = 1
		}
		m.imageDetailOpen = false
		return m.startActiveLoad()
	case key.Matches(msg, m.keys.back):
		if m.imageDetailOpen {
			m.imageDetailOpen = false
		} else if m.imageEditing {
			m.clearImageSelection()
			m.imageEditing = false
		}
		return m, nil
	case key.Matches(msg, m.keys.edit):
		if m.imageDetailOpen {
			return m, nil
		}
		if m.imageEditing {
			m.clearImageSelection()
			m.imageEditing = false
		} else {
			m.imageEditing = true
		}
		return m, nil
	case key.Matches(msg, m.keys.selectRow):
		if m.imageEditing && !m.imageDetailOpen {
			m.toggleImageSelection(m.selectedImageID)
		}
		return m, nil
	case key.Matches(msg, m.keys.retry):
		if m.imageDetailOpen {
			return m, nil
		}
		m.imagesGen++
		m.imagesLoading = true
		m.imagesErr = nil
		return m, m.loadImages(m.imagesGen)
	case key.Matches(msg, m.keys.up):
		if !m.imageDetailOpen {
			m = m.moveImageSelection(-1)
		}
		return m, nil
	case key.Matches(msg, m.keys.down):
		if !m.imageDetailOpen {
			m = m.moveImageSelection(1)
		}
		return m, nil
	case key.Matches(msg, m.keys.confirm):
		if m.imageDetailOpen || m.selectedImageID == "" {
			return m, nil
		}
		if m.imageEditing && len(m.selectedImages) > 0 {
			m.action = actionState{stage: actionMenu, resource: actionImages, targets: m.selectedImageTargets()}
			return m, nil
		}
		m.imageDetailOpen = true
		m.imageDetailLoading = true
		m.imageDetailErr = nil
		m.imageDetails = domain.ImageDetails{}
		id := m.selectedImageID
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
			defer cancel()
			details, err := m.service.ImageDetails(ctx, id)
			return imageDetailsLoadedMsg{details: details, err: err}
		}
	}
	return m, nil
}

func (m Model) startNetworkLoad() (tea.Model, tea.Cmd) {
	if m.active != 3 || m.networksLoaded || m.networksLoading {
		return m, nil
	}
	m.networksGen++
	m.networksLoading = true
	return m, m.loadNetworks(m.networksGen)
}

func (m Model) loadNetworks(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		networks, err := m.service.Networks(ctx)
		return networksLoadedMsg{networks: networks, err: err, generation: generation}
	}
}

func (m Model) updateNetworks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.next):
		m.active, m.networkDetailOpen = 4, false
		return m.startActiveLoad()
	case key.Matches(msg, m.keys.prev):
		m.active, m.networkDetailOpen = 2, false
		return m.startActiveLoad()
	case key.Matches(msg, m.keys.back):
		if m.networkDetailOpen {
			m.networkDetailOpen = false
		} else if m.networkEditing {
			m.clearNetworkSelection()
			m.networkEditing = false
		}
	case key.Matches(msg, m.keys.edit):
		if m.networkDetailOpen {
			return m, nil
		}
		if m.networkEditing {
			m.clearNetworkSelection()
			m.networkEditing = false
		} else {
			m.networkEditing = true
		}
	case key.Matches(msg, m.keys.selectRow):
		if m.networkEditing && !m.networkDetailOpen {
			m.toggleNetworkSelection(m.selectedNetworkID)
		}
	case key.Matches(msg, m.keys.retry):
		if !m.networkDetailOpen {
			m.networksGen++
			m.networksLoading, m.networksErr = true, nil
			return m, m.loadNetworks(m.networksGen)
		}
	case key.Matches(msg, m.keys.up):
		if !m.networkDetailOpen {
			m = m.moveNetworkSelection(-1)
		}
	case key.Matches(msg, m.keys.down):
		if !m.networkDetailOpen {
			m = m.moveNetworkSelection(1)
		}
	case key.Matches(msg, m.keys.confirm):
		if m.networkDetailOpen || m.selectedNetworkID == "" {
			return m, nil
		}
		if m.networkEditing && len(m.selectedNetworks) > 0 {
			m.action = actionState{stage: actionMenu, resource: actionNetworks, targets: m.selectedNetworkTargets()}
			return m, nil
		}
		if !m.networkDetailOpen {
			m.networkDetailOpen, m.networkDetailLoading, m.networkDetailErr, m.networkDetails = true, true, nil, domain.NetworkDetails{}
			id := m.selectedNetworkID
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
				defer cancel()
				details, err := m.service.NetworkDetails(ctx, id)
				return networkDetailsLoadedMsg{details: details, err: err}
			}
		}
	}
	return m, nil
}

func (m *Model) syncNetworkSelection() {
	available := make(map[string]struct{}, len(m.networks))
	for _, network := range m.networks {
		available[network.ID] = struct{}{}
	}
	for id := range m.selectedNetworks {
		if _, found := available[id]; !found {
			delete(m.selectedNetworks, id)
		}
	}
	if _, found := available[m.selectedNetworkID]; found {
		return
	}
	if len(m.networks) == 0 {
		m.selectedNetworkID = ""
		return
	}
	m.selectedNetworkID = m.networks[0].ID
}
func (m Model) moveNetworkSelection(delta int) Model {
	if m.networksLoading || m.networksErr != nil || len(m.networks) == 0 {
		return m
	}
	index := 0
	for candidate, network := range m.networks {
		if network.ID == m.selectedNetworkID {
			index = candidate
			break
		}
	}
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(m.networks) {
		index = len(m.networks) - 1
	}
	m.selectedNetworkID = m.networks[index].ID
	return m
}

func (m Model) startVolumeLoad() (tea.Model, tea.Cmd) {
	if m.active != 4 || m.volumesLoaded || m.volumesLoading {
		return m, nil
	}
	m.volumesGen++
	m.volumesLoading = true
	return m, m.loadVolumes(m.volumesGen)
}
func (m Model) loadVolumes(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		volumes, err := m.service.Volumes(ctx)
		return volumesLoadedMsg{volumes: volumes, err: err, generation: generation}
	}
}
func (m Model) updateVolumes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.next):
		m.active, m.volumeDetailOpen = 0, false
		return m, nil
	case key.Matches(msg, m.keys.prev):
		m.active, m.volumeDetailOpen = 3, false
		return m.startActiveLoad()
	case key.Matches(msg, m.keys.back):
		if m.volumeDetailOpen {
			m.volumeDetailOpen = false
		} else if m.volumeEditing {
			m.clearVolumeSelection()
			m.volumeEditing = false
		}
	case key.Matches(msg, m.keys.edit):
		if m.volumeDetailOpen {
			return m, nil
		}
		if m.volumeEditing {
			m.clearVolumeSelection()
			m.volumeEditing = false
		} else {
			m.volumeEditing = true
		}
	case key.Matches(msg, m.keys.selectRow):
		if m.volumeEditing && !m.volumeDetailOpen {
			m.toggleVolumeSelection(m.selectedVolumeName)
		}
	case key.Matches(msg, m.keys.retry):
		if !m.volumeDetailOpen {
			m.volumesGen++
			m.volumesLoading, m.volumesErr = true, nil
			return m, m.loadVolumes(m.volumesGen)
		}
	case key.Matches(msg, m.keys.up):
		if !m.volumeDetailOpen {
			m = m.moveVolumeSelection(-1)
		}
	case key.Matches(msg, m.keys.down):
		if !m.volumeDetailOpen {
			m = m.moveVolumeSelection(1)
		}
	case key.Matches(msg, m.keys.confirm):
		if m.volumeDetailOpen || m.selectedVolumeName == "" {
			return m, nil
		}
		if m.volumeEditing && len(m.selectedVolumes) > 0 {
			m.action = actionState{stage: actionMenu, resource: actionVolumes, targets: m.selectedVolumeTargets()}
			return m, nil
		}
		if !m.volumeDetailOpen {
			m.volumeDetailOpen, m.volumeDetailLoading, m.volumeDetailErr, m.volumeDetails = true, true, nil, domain.VolumeDetails{}
			name := m.selectedVolumeName
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
				defer cancel()
				details, err := m.service.VolumeDetails(ctx, name)
				return volumeDetailsLoadedMsg{details: details, err: err}
			}
		}
	}
	return m, nil
}
func (m *Model) syncVolumeSelection() {
	available := make(map[string]struct{}, len(m.volumes))
	for _, volume := range m.volumes {
		available[volume.Name] = struct{}{}
	}
	for name := range m.selectedVolumes {
		if _, found := available[name]; !found {
			delete(m.selectedVolumes, name)
		}
	}
	if _, found := available[m.selectedVolumeName]; found {
		return
	}
	if len(m.volumes) == 0 {
		m.selectedVolumeName = ""
		return
	}
	m.selectedVolumeName = m.volumes[0].Name
}
func (m Model) moveVolumeSelection(delta int) Model {
	if m.volumesLoading || m.volumesErr != nil || len(m.volumes) == 0 {
		return m
	}
	index := 0
	for candidate, volume := range m.volumes {
		if volume.Name == m.selectedVolumeName {
			index = candidate
			break
		}
	}
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(m.volumes) {
		index = len(m.volumes) - 1
	}
	m.selectedVolumeName = m.volumes[index].Name
	return m
}

func (m *Model) syncImageSelection() {
	for _, image := range m.images {
		if image.ID == m.selectedImageID {
			return
		}
	}
	if len(m.images) == 0 {
		m.selectedImageID = ""
		return
	}
	m.selectedImageID = m.images[0].ID
}

func (m *Model) syncStackSelection() {
	selected, expanded := false, false
	for _, stack := range m.stacks {
		selected = selected || stack.Name == m.selectedStackName
		expanded = expanded || stack.Name == m.expandedStackName
	}
	if !selected {
		m.selectedStackName = ""
		if len(m.stacks) > 0 {
			m.selectedStackName = m.stacks[0].Name
		}
	}
	if !expanded {
		m.expandedStackName = ""
		m.selectedStackContainerID = ""
		m.stackContainerEditing = false
		m.clearStackContainerSelection()
	}
	available := make(map[string]struct{}, len(m.stacks))
	for _, stack := range m.stacks {
		available[stack.Name] = struct{}{}
	}
	for name := range m.selectedStacks {
		if _, found := available[name]; !found {
			delete(m.selectedStacks, name)
		}
	}
	availableContainers := make(map[string]struct{})
	for _, stack := range m.stacks {
		if stack.Name != m.expandedStackName {
			continue
		}
		for _, container := range stack.ContainerItems {
			availableContainers[container.ID] = struct{}{}
		}
	}
	for id := range m.selectedStackContainers {
		if _, found := availableContainers[id]; !found {
			delete(m.selectedStackContainers, id)
		}
	}
	if _, found := availableContainers[m.selectedStackContainerID]; !found {
		m.selectedStackContainerID = ""
	}
}

func (m *Model) clearStackSelection() { m.selectedStacks = make(map[string]struct{}) }
func (m *Model) toggleStackSelection(name string) {
	if name == "" {
		return
	}
	if _, selected := m.selectedStacks[name]; selected {
		delete(m.selectedStacks, name)
		return
	}
	m.selectedStacks[name] = struct{}{}
}
func (m Model) selectedStackTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selectedStacks))
	for _, stack := range m.stacks {
		if _, selected := m.selectedStacks[stack.Name]; selected {
			targets = append(targets, actionTarget{ID: stack.Name, Name: stack.Name, State: stack.State, Unavailable: stack.DownUnavailableReason()})
		}
	}
	return targets
}

func (m Model) moveStackSelection(delta int) Model {
	if m.stacksLoading || m.stacksErr != nil || len(m.stacks) == 0 {
		return m
	}
	type item struct {
		stack     string
		container string
	}
	items := make([]item, 0, len(m.stacks))
	for _, stack := range m.stacks {
		items = append(items, item{stack: stack.Name})
		if stack.Name == m.expandedStackName {
			for _, container := range stack.ContainerItems {
				items = append(items, item{stack: stack.Name, container: container.ID})
			}
		}
	}
	index := 0
	for candidate, item := range items {
		if item.container != "" && item.container == m.selectedStackContainerID || item.container == "" && m.selectedStackContainerID == "" && item.stack == m.selectedStackName {
			index = candidate
			break
		}
	}
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	m.selectedStackName = items[index].stack
	m.selectedStackContainerID = items[index].container
	return m
}

func (m Model) selectedStack() *domain.Stack {
	for index := range m.stacks {
		if m.stacks[index].Name == m.selectedStackName {
			return &m.stacks[index]
		}
	}
	return nil
}

func (m *Model) clearStackContainerSelection() { m.selectedStackContainers = make(map[string]struct{}) }
func (m *Model) toggleStackContainerSelection(id string) {
	if _, selected := m.selectedStackContainers[id]; selected {
		delete(m.selectedStackContainers, id)
		return
	}
	m.selectedStackContainers[id] = struct{}{}
}
func (m Model) selectedStackContainerTargets() []actionTarget {
	ids := make([]string, 0, len(m.selectedStackContainers))
	for id := range m.selectedStackContainers {
		ids = append(ids, id)
	}
	return m.selectedStackContainerTargetsForIDs(ids)
}
func (m Model) selectedStackContainerTargetsForIDs(ids []string) []actionTarget {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	var targets []actionTarget
	if stack := m.selectedStack(); stack != nil {
		for _, container := range stack.ContainerItems {
			if _, ok := selected[container.ID]; ok {
				targets = append(targets, actionTarget{ID: container.ID, Name: container.Name, State: container.State})
			}
		}
	}
	return targets
}

func (m *Model) clearImageSelection() {
	m.selectedImages = make(map[string]struct{})
}

func (m *Model) clearNetworkSelection() {
	m.selectedNetworks = make(map[string]struct{})
}

func (m *Model) toggleNetworkSelection(id string) {
	if id == "" {
		return
	}
	if _, selected := m.selectedNetworks[id]; selected {
		delete(m.selectedNetworks, id)
		return
	}
	m.selectedNetworks[id] = struct{}{}
}

func (m Model) selectedNetworkTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selectedNetworks))
	for _, network := range m.networks {
		if _, selected := m.selectedNetworks[network.ID]; selected {
			targets = append(targets, actionTarget{ID: network.ID, Name: network.Name})
		}
	}
	return targets
}

func (m *Model) clearVolumeSelection() {
	m.selectedVolumes = make(map[string]struct{})
}

func (m *Model) toggleVolumeSelection(name string) {
	if name == "" {
		return
	}
	if _, selected := m.selectedVolumes[name]; selected {
		delete(m.selectedVolumes, name)
		return
	}
	m.selectedVolumes[name] = struct{}{}
}

func (m Model) selectedVolumeTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selectedVolumes))
	for _, volume := range m.volumes {
		if _, selected := m.selectedVolumes[volume.Name]; selected {
			targets = append(targets, actionTarget{ID: volume.Name, Name: volume.Name})
		}
	}
	return targets
}

func (m *Model) toggleImageSelection(id string) {
	if id == "" {
		return
	}
	if _, selected := m.selectedImages[id]; selected {
		delete(m.selectedImages, id)
		return
	}
	m.selectedImages[id] = struct{}{}
}

func (m Model) selectedImageTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selectedImages))
	for _, image := range m.images {
		if _, selected := m.selectedImages[image.ID]; selected {
			targets = append(targets, actionTarget{ID: image.ID, Name: image.Name})
		}
	}
	return targets
}

func (m Model) moveImageSelection(delta int) Model {
	if m.imagesLoading || m.imagesErr != nil || len(m.images) == 0 {
		return m
	}
	index := 0
	for candidate, image := range m.images {
		if image.ID == m.selectedImageID {
			index = candidate
			break
		}
	}
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(m.images) {
		index = len(m.images) - 1
	}
	m.selectedImageID = m.images[index].ID
	return m
}

func (m Model) openDetails() (tea.Model, tea.Cmd) {
	if m.selectedID == "" {
		return m, nil
	}
	m.panel = panelDetails
	m.details = domain.ContainerDetails{}
	m.detailErr = nil
	m.detailLoading = true
	id := m.selectedID
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		details, err := m.service.Details(ctx, id)
		return detailsLoadedMsg{details: details, err: err}
	}
}

func (m Model) startLogs() (tea.Model, tea.Cmd) {
	if m.selectedID == "" {
		return m, nil
	}
	return m.startLogsFor(m.selectedID)
}

func (m Model) startShellFor(containerID string) (tea.Model, tea.Cmd) {
	if containerID == "" {
		return m, nil
	}
	if m.snapshot.Engine.Remote {
		m.shellErr = domain.ErrRemoteUnsupported
		return m, nil
	}
	m.shellActive = true
	m.shellErr = nil
	command := makeShellCommand(containerID)
	return m, tea.ExecProcess(command, func(err error) tea.Msg {
		return shellFinishedMsg{err: err}
	})
}

func (m Model) startLogsFor(id string) (tea.Model, tea.Cmd) {
	if id == "" {
		return m, nil
	}
	m.closeLogs()
	m.panel = panelLogs
	m.logLines = nil
	m.logErr = nil
	m.logOffset = 0
	m.stackLogs = false
	m.logTitle = ""
	m.logGen++
	ctx, cancel := context.WithCancel(m.ctx)
	m.logCancel = cancel
	gen := m.logGen
	return m, func() tea.Msg {
		stream, err := m.service.Logs(ctx, id, 100)
		return logsOpenedMsg{stream: stream, err: err, gen: gen}
	}
}

func (m Model) startStackLogs(stack domain.Stack) (tea.Model, tea.Cmd) {
	m.closeLogs()
	m.panel, m.stackLogs, m.logTitle = panelLogs, true, stack.Name
	m.logLines, m.logErr, m.logOffset = nil, nil, 0
	m.logGen++
	ctx, cancel := context.WithCancel(m.ctx)
	m.logCancel = cancel
	gen := m.logGen
	return m, func() tea.Msg {
		stream, err := m.service.ComposeLogs(ctx, stack, 100)
		return logsOpenedMsg{stream: stream, err: err, gen: gen}
	}
}

func (m *Model) closeLogs() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	m.logActive = false
	m.logGen++
}

func (m Model) waitLog(stream ports.LogStream, gen uint64) tea.Cmd {
	return func() tea.Msg {
		select {
		case line, ok := <-stream.Lines:
			if !ok {
				return logsClosedMsg{gen: gen}
			}
			return logLineMsg{line: line, stream: stream, gen: gen}
		case err, ok := <-stream.Errors:
			if !ok {
				return logsClosedMsg{gen: gen}
			}
			return logErrorMsg{err: err, gen: gen}
		}
	}
}

func (m Model) updateLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.back):
		m.closeLogs()
		if m.stackLogs {
			m.panel = panelContainers
			m.active = 1
			m.stackLogs = false
		} else {
			m.panel = panelContainers
		}
	case key.Matches(msg, m.keys.up):
		if m.logOffset < len(m.logLines)-1 {
			m.logOffset++
		}
	case key.Matches(msg, m.keys.down):
		if m.logOffset > 0 {
			m.logOffset--
		}
	}
	return m, nil
}

func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(now time.Time) tea.Msg {
		return refreshMsg(now)
	})
}

func (m Model) updateActionMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.back):
		m.action = actionState{}
	case key.Matches(msg, m.keys.up):
		if m.action.index > 0 {
			m.action.index--
		}
	case key.Matches(msg, m.keys.down):
		if m.action.index < len(m.actionChoices())-1 {
			m.action.index++
		}
	case key.Matches(msg, m.keys.confirm):
		if m.selectedAction() == "cancel" {
			m.action = actionState{}
		} else {
			m.action.stage = actionConfirm
		}
	}

	return m, nil
}

func (m Model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.back) {
		m.action.stage = actionMenu
		m.action.input = ""
		return m, nil
	}

	action := m.selectedAction()
	if m.action.resource == actionStacks {
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
			m.action.running = true
			return m, m.runAction(action)
		}
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'n' || msg.Runes[0] == 'N') {
			m.action = actionState{}
		}
		return m, nil
	}
	if action == application.ActionDelete {
		switch msg.Type {
		case tea.KeyBackspace:
			if len(m.action.input) > 0 {
				m.action.input = m.action.input[:len(m.action.input)-1]
			}
		case tea.KeyRunes:
			m.action.input += string(msg.Runes)
		case tea.KeyEnter:
			if m.action.input == "delete" {
				m.action.running = true
				return m, m.runAction(action)
			}
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.confirm) {
		m.action.running = true
		return m, m.runAction(action)
	}

	return m, nil
}

func (m Model) runAction(action application.Action) tea.Cmd {
	targets := append([]actionTarget(nil), m.action.targets...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 45*time.Second)
		defer cancel()

		ids := make([]string, 0, len(targets))
		for _, target := range targets {
			ids = append(ids, target.ID)
		}
		if m.action.resource == actionImages {
			return actionFinishedMsg{results: m.service.RemoveImages(ctx, ids)}
		}
		if m.action.resource == actionNetworks {
			return actionFinishedMsg{results: m.service.RemoveNetworks(ctx, ids)}
		}
		if m.action.resource == actionVolumes {
			return actionFinishedMsg{results: m.service.RemoveVolumes(ctx, ids)}
		}
		if m.action.resource == actionStacks {
			stacks := make([]domain.Stack, 0, len(targets))
			for _, target := range targets {
				for _, stack := range m.stacks {
					if stack.Name == target.ID {
						stacks = append(stacks, stack)
						break
					}
				}
			}
			return actionFinishedMsg{results: m.service.ActStacks(ctx, action, stacks)}
		}
		return actionFinishedMsg{results: m.service.Act(ctx, action, ids)}
	}
}

func (m Model) actionMenuView() sharedui.View {
	choices := m.actionChoices()
	lines := make([]string, 0, len(choices))
	for index, choice := range choices {
		marker := " "
		if index == m.action.index {
			marker = ">"
		}
		lines = append(lines, marker+" "+actionLabelForResource(choice, m.action.resource))
	}

	return sharedui.View{
		Title:      actionResourceLabel(m.action.resource),
		Status:     sharedui.StatusWarning,
		HideStatus: true,
		Sections: []sharedui.Section{
			{Title: fmt.Sprintf("Selected: %d %s", len(m.action.targets), strings.ToLower(actionResourceLabel(m.action.resource))), Body: strings.Join(m.targetNames(m.action.targets), "\n")},
			{Title: "Action", Body: strings.Join(lines, "\n")},
			{Title: "Controls", Body: "up/down choose | enter continue | esc cancel"},
		},
	}
}

func (m Model) actionConfirmationView() sharedui.View {
	action := m.selectedAction()
	title := strings.ToUpper(string(action)) + fmt.Sprintf(" %d %s", len(m.action.targets), strings.ToLower(actionResourceLabel(m.action.resource)))
	body := strings.Join(m.targetNames(m.action.targets), "\n")
	engine := engineTarget(m.snapshot.Engine)
	if m.action.resource == actionStacks {
		return sharedui.View{Title: "Stacks", Status: sharedui.StatusWarning, HideStatus: true, Sections: []sharedui.Section{
			{Title: "Confirm " + actionLabel(action), Body: title + " on " + engine + "\n\n" + body + stackConfirmationWarning(action)},
			{Title: "Confirmation", Body: "Are you sure? [y/N]"},
			{Title: "Controls", Body: "y confirm | n or esc cancel"},
		}}
	}
	if action == application.ActionDelete {
		return sharedui.View{
			Title:      actionResourceLabel(m.action.resource),
			Status:     sharedui.StatusError,
			HideStatus: true,
			Sections: []sharedui.Section{
				{Title: deleteTitle(m.action.resource), Body: title + " on " + engine + "\n\n" + body},
				{Title: "Confirmation", Body: deleteConfirmationText(m.action.resource) + "\n\n> " + m.action.input},
				{Title: "Controls", Body: "enter confirm | esc back"},
			},
		}
	}

	return sharedui.View{
		Title:      actionResourceLabel(m.action.resource),
		Status:     sharedui.StatusWarning,
		HideStatus: true,
		Sections: []sharedui.Section{
			{Title: "Confirm " + actionLabel(action), Body: title + " on " + engine + "\n\n" + body},
			{Title: "Controls", Body: "enter confirm | esc back"},
		},
	}
}

func (m Model) actionResultView() sharedui.View {
	lines := make([]string, 0, len(m.action.results))
	for _, result := range m.action.results {
		name := result.ID
		for _, target := range m.action.targets {
			if target.ID == result.ID {
				name = target.Name
				break
			}
		}
		if result.Err != nil {
			lines = append(lines, "FAIL  "+name+": "+result.Err.Error())
			continue
		}
		lines = append(lines, "OK    "+name)
	}

	return sharedui.View{
		Title:      actionResourceLabel(m.action.resource),
		Status:     actionResultStatus(m.action.results),
		HideStatus: true,
		Sections: []sharedui.Section{
			{Title: "Action results", Body: strings.Join(lines, "\n")},
			{Title: "Controls", Body: "enter or esc close"},
		},
	}
}

func (m *Model) toggleSelection(id string) {
	if id == "" {
		return
	}
	if _, selected := m.selected[id]; selected {
		delete(m.selected, id)
		return
	}
	m.selected[id] = struct{}{}
}

func (m *Model) clearSelection() {
	m.selected = make(map[string]struct{})
}

func (m Model) selectedTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selected))
	for _, container := range m.snapshot.Containers {
		if _, selected := m.selected[container.ID]; selected {
			targets = append(targets, actionTarget{ID: container.ID, Name: container.Name, State: container.State})
		}
	}

	return targets
}

func (m Model) syncSelection() Model {
	if len(m.snapshot.Containers) == 0 {
		m.selectedID = ""
		m.clearSelection()
		return m
	}

	available := make(map[string]struct{}, len(m.snapshot.Containers))
	for _, container := range m.snapshot.Containers {
		available[container.ID] = struct{}{}
	}
	for id := range m.selected {
		if _, found := available[id]; !found {
			delete(m.selected, id)
		}
	}

	for _, container := range m.snapshot.Containers {
		if container.ID == m.selectedID {
			return m
		}
	}
	m.selectedID = m.snapshot.Containers[0].ID

	return m
}

func actionChoices(resource actionResource) []application.Action {
	if resource == actionStacks {
		return []application.Action{application.ActionDown, "cancel"}
	}
	if resource == actionStackContainers {
		return []application.Action{application.ActionRestart, application.ActionStop, "cancel"}
	}
	if resource == actionImages || resource == actionNetworks || resource == actionVolumes {
		return []application.Action{application.ActionDelete, "cancel"}
	}
	return []application.Action{application.ActionStop, application.ActionRestart, application.ActionDelete, "cancel"}
}

func stackActionChoices(targets []actionTarget) []application.Action {
	if len(targets) == 0 {
		return []application.Action{"cancel"}
	}
	available := func() bool {
		for _, target := range targets {
			if target.Unavailable != "" {
				return false
			}
		}
		return true
	}
	if !available() {
		return []application.Action{"cancel"}
	}
	state := strings.ToLower(targets[0].State)
	for _, target := range targets[1:] {
		if strings.ToLower(target.State) != state {
			return []application.Action{"cancel"}
		}
	}
	switch state {
	case "down", "missing compose file":
		return []application.Action{application.ActionUp, "cancel"}
	case "running", "mixed":
		return []application.Action{application.ActionStop, application.ActionRestart, application.ActionDown, "cancel"}
	case "stopped":
		return []application.Action{application.ActionUp, application.ActionRestart, application.ActionDown, "cancel"}
	default:
		return []application.Action{"cancel"}
	}
}

func (m Model) actionChoices() []application.Action {
	if len(m.action.choices) > 0 {
		return m.action.choices
	}
	return actionChoices(m.action.resource)
}

func (m Model) selectedAction() application.Action {
	choices := m.actionChoices()
	if m.action.index < 0 || m.action.index >= len(choices) {
		return "cancel"
	}
	return choices[m.action.index]
}

func selectedAction(resource actionResource, index int) application.Action {
	choices := actionChoices(resource)
	if index < 0 || index >= len(choices) {
		return "cancel"
	}

	return choices[index]
}

func actionResourceLabel(resource actionResource) string {
	switch resource {
	case actionImages:
		return "Images"
	case actionNetworks:
		return "Networks"
	case actionVolumes:
		return "Volumes"
	case actionStacks:
		return "Stacks"
	case actionStackContainers:
		return "Stack containers"
	}
	return "Containers"
}

func deleteTitle(resource actionResource) string {
	switch resource {
	case actionImages:
		return "DELETE IMAGES"
	case actionNetworks:
		return "DELETE NETWORKS"
	case actionVolumes:
		return "DELETE VOLUMES"
	}
	return "FORCE DELETE"
}

func deleteConfirmationText(resource actionResource) string {
	switch resource {
	case actionImages:
		return "Type delete to remove images without force. Removal fails for images used by containers."
	case actionNetworks:
		return "Type delete to remove networks without force. Removal fails if a network is connected."
	case actionVolumes:
		return "Type delete to remove volumes without force. Removal can delete persistent data and fails if a volume is referenced."
	}
	return "Type delete to force removal. Volumes will not be removed."
}

func actionLabel(action application.Action) string {
	switch action {
	case application.ActionStop:
		return "Stop"
	case application.ActionRestart:
		return "Restart"
	case application.ActionDelete:
		return "Force delete"
	case application.ActionDown:
		return "Down stack"
	case application.ActionUp:
		return "Up stack"
	default:
		return "Cancel"
	}
}

func actionLabelForResource(action application.Action, resource actionResource) string {
	if (resource == actionImages || resource == actionNetworks || resource == actionVolumes) && action == application.ActionDelete {
		return "Delete"
	}
	return actionLabel(action)
}

func (m Model) actionView() sharedui.View {
	switch m.action.stage {
	case actionMenu:
		return m.actionMenuView()
	case actionConfirm:
		return m.actionConfirmationView()
	default:
		return m.actionResultView()
	}
}

func (m Model) targetNames(targets []actionTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		line := "- " + target.Name
		if target.Unavailable != "" {
			line += " (unavailable: " + target.Unavailable + ")"
		} else if m.action.resource == actionStacks && target.State != "" {
			line += " (unavailable: " + target.State + ")"
		}
		names = append(names, line)
	}

	return names
}

func shortContainerID(id string) string {
	if len(id) <= 12 {
		return id
	}

	return id[:12]
}

func engineTarget(engine domain.EngineInfo) string {
	scope := "LOCAL"
	if engine.Remote {
		scope = "REMOTE"
	}

	return scope + " " + engine.Name
}

func actionResultStatus(results []application.ActionResult) sharedui.Status {
	for _, result := range results {
		if result.Err != nil {
			return sharedui.StatusWarning
		}
	}

	return sharedui.StatusReady
}

func (m Model) containersView(layout sharedui.Layout) sharedui.View {
	if m.action.stage == actionMenu {
		if m.action.resource == actionImages {
			return m.imagesActionView()
		}
		return m.actionMenuView()
	}
	if m.action.stage == actionConfirm {
		if m.action.resource == actionImages {
			return m.imagesActionView()
		}
		return m.actionConfirmationView()
	}
	if m.action.stage == actionResult {
		if m.action.resource == actionImages {
			return m.imagesActionView()
		}
		return m.actionResultView()
	}
	if m.panel == panelDetails {
		return m.detailsView()
	}
	if m.panel == panelLogs {
		return m.logsView(layout)
	}
	if m.shellActive {
		return sharedui.View{Title: "Container shell", Status: sharedui.StatusLoading, Summary: "Starting an interactive shell..."}
	}
	if m.shellErr != nil {
		return sharedui.View{
			Title: "Containers", Status: sharedui.StatusError, Summary: "Container shell failed: " + m.shellErr.Error(),
			Sections: []sharedui.Section{{Title: "Next", Body: "Check that the container is running, /bin/sh exists, and Docker permissions allow exec. Press s to try again."}},
		}
	}
	if m.showHelp {
		return sharedui.View{
			Title:      "Containers",
			Status:     sharedui.StatusReady,
			HideStatus: true,
			Sections: []sharedui.Section{
				{Title: "Help", Body: "s shell | e edit | space select | enter actions | up/down move | o sort | r refresh | left/right view | esc close | q quit"},
				{Title: "Connection", Body: engineDetails(m.snapshot.Engine)},
			},
		}
	}

	if m.loading {
		return sharedui.View{
			Title:   "Containers",
			Status:  sharedui.StatusLoading,
			Summary: "Connecting to Docker Engine and loading containers...",
		}
	}

	if m.err != nil {
		status := sharedui.StatusError
		if errors.Is(m.err, domain.ErrRemoteUnsupported) {
			status = sharedui.StatusUnavailable
		}

		return sharedui.View{
			Title:   "Containers",
			Status:  status,
			Summary: dockerErrorSummary(m.err),
			Sections: []sharedui.Section{
				{Title: "Connection", Body: "dtop could not connect to a supported local Docker Engine."},
				{Title: "Next", Body: "Check Docker Desktop, docker context, DOCKER_HOST, socket permissions, or daemon status. Press r to retry."},
			},
		}
	}

	if len(m.snapshot.Containers) == 0 {
		return sharedui.View{
			Title:   "Containers",
			Status:  sharedui.StatusEmpty,
			Summary: "Connected to Docker Engine, but no containers were found.",
			Sections: []sharedui.Section{
				{Title: "Containers", Body: "No containers. Try creating one with Docker, then press r to retry."},
			},
		}
	}

	return sharedui.View{
		Title:      "Containers",
		Status:     sharedui.StatusReady,
		HideStatus: true,
		Sections: []sharedui.Section{
			{Body: renderContainers(m.snapshot.Containers, m.selectedID, m.selected, m.editing, m.now(), layout, m.memoryMode)},
		},
	}
}

func (m Model) imagesActionView() sharedui.View {
	switch m.action.stage {
	case actionMenu:
		return m.actionMenuView()
	case actionConfirm:
		return m.actionConfirmationView()
	case actionResult:
		return m.actionResultView()
	default:
		return sharedui.View{}
	}
}

func (m Model) detailsView() sharedui.View {
	if m.detailLoading {
		return sharedui.View{Title: "Container details", Status: sharedui.StatusLoading, Summary: "Loading container details..."}
	}
	if m.detailErr != nil {
		return sharedui.View{
			Title: "Container details", Status: sharedui.StatusError, Summary: m.detailErr.Error(),
			Sections: []sharedui.Section{{Title: "Controls", Body: "esc back"}},
		}
	}
	details := m.details
	ports := "-"
	if len(details.Ports) > 0 {
		ports = strings.Join(details.Ports, "\n")
	}
	networks := "-"
	if len(details.Networks) > 0 {
		networks = strings.Join(details.Networks, ", ")
	}
	return sharedui.View{
		Title: "Container details", Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{
			{Title: details.Name, Body: strings.Join([]string{"ID: " + shortContainerID(details.ID), "Image: " + details.Image, "State: " + details.State, "Health: " + details.Health, "Uptime: " + formatUptime(details.StartedAt, details.State, m.now())}, "\n")},
			{Title: "Ports", Body: ports},
			{Title: "Networks", Body: networks},
			{Title: "Controls", Body: "l logs | esc back"},
		},
	}
}

func (m Model) logsView(layout sharedui.Layout) sharedui.View {
	status := "Loading log stream..."
	if m.logActive {
		status = "Following live logs"
	}
	if m.logErr != nil {
		status = m.logErr.Error()
	}
	lines := visibleLogs(m.logLines, m.logOffset, logLineCount(layout))
	body := "(no log output)"
	if len(lines) > 0 {
		body = strings.Join(lines, "\n")
	}
	return sharedui.View{
		Title: logPanelTitle(m.stackLogs), Status: sharedui.StatusReady, HideStatus: true, Summary: status,
		Sections: []sharedui.Section{
			{Title: m.logName(), Body: body},
			{Title: "Controls", Body: "up/down scroll | esc stop and back"},
		},
	}
}

func logPanelTitle(stack bool) string {
	if stack {
		return "Compose logs"
	}
	return "Container logs"
}
func (m Model) logName() string {
	if m.stackLogs {
		return m.logTitle
	}
	return m.selectedContainerName()
}

func stackConfirmationWarning(action application.Action) string {
	if action == application.ActionDown {
		return "\n\nVolumes are not removed."
	}
	return ""
}

func (m Model) selectedContainerName() string {
	for _, container := range m.snapshot.Containers {
		if container.ID == m.selectedID {
			return container.Name
		}
	}
	return shortContainerID(m.selectedID)
}

func logLineCount(layout sharedui.Layout) int {
	count := layout.Height - 10
	if layout.Framed {
		count -= 6
	}
	if count < 1 {
		return 1
	}
	return count
}

func visibleLogs(lines []string, offset, limit int) []string {
	end := len(lines) - offset
	if end < 0 {
		end = 0
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	return lines[start:end]
}

func engineDetails(info domain.EngineInfo) string {
	return strings.Join([]string{
		"Name: " + info.Name,
		"Endpoint: " + info.Endpoint,
		"Transport: " + info.Transport,
		"Remote: " + yesNo(info.Remote),
		"Secure: " + yesNo(info.Secure),
		"Source: " + info.Source,
		"Server: " + info.ServerVersion,
		"API: " + info.APIVersion,
		"OS: " + info.OperatingSystem,
		fmt.Sprintf("CPUs: %d", info.NCPU),
		"RAM: " + formatBytes(info.MemoryTotal),
	}, "\n")
}

func dockerErrorSummary(err error) string {
	if errors.Is(err, domain.ErrRemoteUnsupported) {
		return "A remote Docker endpoint is configured, but D1A only supports local Engines. Remote support is planned for D1R."
	}

	return err.Error()
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

func (m Model) headerSummary() string {
	if m.loading {
		return "connecting Docker Engine"
	}
	if m.err != nil {
		return "Docker unavailable"
	}

	running := 0
	for _, container := range m.snapshot.Containers {
		if container.State == "running" {
			running++
		}
	}

	scope := "LOCAL"
	if m.snapshot.Engine.Remote {
		scope = "REMOTE"
	}
	cpu := "--"
	if m.snapshot.CPUAvailable {
		cpu = fmt.Sprintf("%.1f%%", m.snapshot.ContainerCPUPercent)
	}
	memory := "--/" + formatBytes(m.snapshot.Engine.MemoryTotal)
	if m.snapshot.MemoryAvailable {
		memory = formatBytes(m.snapshot.ContainerMemoryUsage) + "/" + formatBytes(m.snapshot.Engine.MemoryTotal)
	}

	return fmt.Sprintf("%s %s | CPU %s | RAM %s | %d/%d running | SORT: %s | Docker %s", scope, m.snapshot.Engine.Name, cpu, memory, running, len(m.snapshot.Containers), sortLabel(m.sortMode), m.snapshot.Engine.ServerVersion)
}

func (m Model) footer(layout sharedui.Layout) string {
	if m.active == 1 {
		if m.selectedStackContainerID != "" && m.expandedStackName == m.selectedStackName {
			return "s shell  l logs  enter actions  esc collapse  up/down select  r refresh  left/right views  q quit"
		}
		return "enter expand  esc collapse  up/down select  r refresh  left/right views  q quit"
	}
	if m.active == 2 {
		if m.action.resource == actionImages && m.action.stage != actionNone {
			return "up/down choose  enter continue  esc cancel"
		}
		if m.imageDetailOpen {
			return "esc back  left/right views  q quit"
		}
		if m.imageEditing {
			return fmt.Sprintf("EDIT: %d selected | space toggle | enter actions | e/esc cancel", len(m.selectedImages))
		}
		return "enter details  e edit  up/down select  r refresh  left/right views  q quit"
	}
	if m.active == 3 {
		if m.action.resource == actionNetworks && m.action.stage != actionNone {
			return "up/down choose  enter continue  esc cancel"
		}
		if m.networkDetailOpen {
			return "esc back  left/right views  q quit"
		}
		if m.networkEditing {
			return fmt.Sprintf("EDIT: %d selected | space toggle | enter actions | e/esc cancel", len(m.selectedNetworks))
		}
		return "enter details  e edit  up/down select  r refresh  left/right views  q quit"
	}
	if m.active == 4 {
		if m.action.resource == actionVolumes && m.action.stage != actionNone {
			return "up/down choose  enter continue  esc cancel"
		}
		if m.volumeDetailOpen {
			return "esc back  left/right views  q quit"
		}
		if m.volumeEditing {
			return fmt.Sprintf("EDIT: %d selected | space toggle | enter actions | e/esc cancel", len(m.selectedVolumes))
		}
		return "enter details  e edit  up/down select  r refresh  left/right views  q quit"
	}
	if m.panel == panelLogs {
		return "up/down scroll  esc stop and back  q quit"
	}
	if m.panel == panelDetails {
		return "l logs  esc back  q quit"
	}
	if m.editing {
		return fmt.Sprintf("EDIT: %d selected | space toggle | enter actions | e/esc cancel", len(m.selected))
	}
	switch layout.Mode {
	case sharedui.LayoutMinimal:
		return "e edit  q quit  up/down"
	case sharedui.LayoutCompact:
		return "s shell  e edit  up/down select  o sort  r refresh  q quit"
	default:
		return "s shell  enter details  l logs  e edit  up/down select  o sort  r refresh  left/right views  ? help  q quit"
	}
}

func sortLabel(mode application.SortMode) string {
	switch mode {
	case application.SortCPU:
		return "CPU"
	case application.SortMemory:
		return "Memory"
	case application.SortName:
		return "Name"
	default:
		return "State"
	}
}

func (m Model) moveSelection(delta int) Model {
	if m.loading || m.err != nil || m.active != 0 || len(m.snapshot.Containers) == 0 {
		return m
	}

	index := m.selectedIndex()
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(m.snapshot.Containers) {
		index = len(m.snapshot.Containers) - 1
	}
	m.selectedID = m.snapshot.Containers[index].ID

	return m
}

func (m Model) selectedIndex() int {
	for index, container := range m.snapshot.Containers {
		if container.ID == m.selectedID {
			return index
		}
	}

	return 0
}
