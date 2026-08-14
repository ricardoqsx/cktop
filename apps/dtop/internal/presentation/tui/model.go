package tui

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

type Model struct {
	service                  application.ContainerService
	localizer                sharedui.Localizer
	updates                  *application.ImageUpdateService
	updatesStarted           bool
	updatesRunning           bool
	updatesGeneration        uint64
	updatesCancel            context.CancelFunc
	updatesChecking          map[string]domain.UpdateStatus
	containerUpdates         map[string]domain.UpdateStatus
	pendingRecreates         map[string]pendingImageRecreate
	dockerHubLoginCheck      func(context.Context) bool
	dockerHubLoginChecked    bool
	dockerHubLoginConfigured bool
	memoryMode               config.MemoryMode
	accentColor              string
	focusColor               string
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
	resourcesLoading         bool
	resourcesGen             uint64
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
	notice                   string
	noticeGeneration         uint64
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
	advanced                 advancedState
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
	selectAll key.Binding
	selectRow key.Binding
	confirm   key.Binding
	details   key.Binding
	logs      key.Binding
	shell     key.Binding
	advanced  key.Binding
}

type advancedStage int

const (
	advancedClosed advancedStage = iota
	advancedMenu
	advancedConfirm
	advancedRunning
	advancedResult
)

type advancedState struct {
	stage  advancedStage
	index  int
	input  string
	result application.PruneResult
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
	ID            string
	Name          string
	State         string
	ImageID       string
	Update        domain.UpdateStatus
	UpdatePending bool
	UpdateUnknown bool
	Unavailable   string
	PullRefs      []string
	Recreate      []application.RecreateTarget
	Stack         *domain.Stack
	Service       string
}

type actionState struct {
	stage    actionStage
	resource actionResource
	index    int
	targets  []actionTarget
	running  bool
	results  []application.ActionResult
	choices  []application.Action
}

type pendingImageRecreate struct {
	ContainerID string
	ImageID     string
	Reference   string
	Compose     bool
}

type loadedMsg struct {
	snapshot   domain.Snapshot
	err        error
	generation uint64
}

type refreshMsg time.Time

type resourceRefreshMsg time.Time

type actionFinishedMsg struct {
	results  []application.ActionResult
	resource actionResource
	action   application.Action
	targets  []actionTarget
}

type actionNoticeExpiredMsg struct{ generation uint64 }

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
	resources   ports.ResourceLoad
	err         error
	generation  uint64
	imagesGen   uint64
	networksGen uint64
	volumesGen  uint64
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
type imageUpdatesLoadedMsg struct {
	updates    []domain.ImageUpdate
	generation uint64
}
type imageUpdateRefreshMsg struct{ generation uint64 }
type dockerHubLoginLoadedMsg struct{ configured bool }
type advancedFinishedMsg struct{ result application.PruneResult }

const (
	refreshInterval         = 2 * time.Second
	resourceRefreshInterval = 5 * time.Second
	actionResultDuration    = 5 * time.Second
)

func NewModel(service application.ContainerService, memoryMode config.MemoryMode, stackDiagnostics ...string) Model {
	return NewModelWithDisplay(service, config.Display{MemoryMode: memoryMode, AccentColor: "63", FocusColor: "15"}, stackDiagnostics...)
}

func NewModelWithDisplay(service application.ContainerService, display config.Display, stackDiagnostics ...string) Model {
	return newModel(service, display, nil, nil, nil, stackDiagnostics...)
}

func NewModelWithUpdates(service application.ContainerService, display config.Display, updates *application.ImageUpdateService, dockerHubLoginCheck func(context.Context) bool, stackDiagnostics ...string) Model {
	return newModel(service, display, updates, dockerHubLoginCheck, nil, stackDiagnostics...)
}

func NewModelWithUpdatesAndLocalizer(service application.ContainerService, display config.Display, updates *application.ImageUpdateService, dockerHubLoginCheck func(context.Context) bool, localizer sharedui.Localizer, stackDiagnostics ...string) Model {
	return newModel(service, display, updates, dockerHubLoginCheck, localizer, stackDiagnostics...)
}

func newModel(service application.ContainerService, display config.Display, updates *application.ImageUpdateService, dockerHubLoginCheck func(context.Context) bool, localizer sharedui.Localizer, stackDiagnostics ...string) Model {
	if localizer == nil {
		localizer = i18n.New("en")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		service:                 service,
		localizer:               localizer,
		updates:                 updates,
		dockerHubLoginCheck:     dockerHubLoginCheck,
		memoryMode:              display.MemoryMode,
		accentColor:             display.AccentColor,
		focusColor:              display.FocusColor,
		loading:                 true,
		stacksLoading:           true,
		stackDiagnostics:        append([]string(nil), stackDiagnostics...),
		imagesLoading:           true,
		networksLoading:         true,
		volumesLoading:          true,
		generation:              1,
		resourcesLoading:        true,
		resourcesGen:            1,
		sortMode:                application.SortState,
		selected:                make(map[string]struct{}),
		selectedImages:          make(map[string]struct{}),
		selectedNetworks:        make(map[string]struct{}),
		selectedVolumes:         make(map[string]struct{}),
		selectedStacks:          make(map[string]struct{}),
		selectedStackContainers: make(map[string]struct{}),
		updatesChecking:         make(map[string]domain.UpdateStatus),
		containerUpdates:        make(map[string]domain.UpdateStatus),
		pendingRecreates:        make(map[string]pendingImageRecreate),
		keys: keyMap{
			quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", localizer.Text(i18n.MessageKeyQuit))),
			next:      key.NewBinding(key.WithKeys("right"), key.WithHelp("right", localizer.Text(i18n.MessageKeyNext))),
			prev:      key.NewBinding(key.WithKeys("left"), key.WithHelp("left", localizer.Text(i18n.MessageKeyPrevious))),
			retry:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", localizer.Text(i18n.MessageKeyRetry))),
			up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", localizer.Text(i18n.MessageKeyUp))),
			down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", localizer.Text(i18n.MessageKeyDown))),
			help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", localizer.Text(i18n.MessageKeyHelp))),
			back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", localizer.Text(i18n.MessageKeyBack))),
			sort:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", localizer.Text(i18n.MessageKeySort))),
			edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", localizer.Text(i18n.MessageKeyEdit))),
			selectAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", localizer.Text(i18n.MessageKeyAll))),
			selectRow: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", localizer.Text(i18n.MessageKeySelect))),
			confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", localizer.Text(i18n.MessageKeyActions))),
			details:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", localizer.Text(i18n.MessageKeyDetails))),
			logs:      key.NewBinding(key.WithKeys("l"), key.WithHelp("l", localizer.Text(i18n.MessageKeyLogs))),
			shell:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", localizer.Text(i18n.MessageKeyShell))),
			advanced:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", localizer.Text(i18n.MessageKeyAdvanced))),
		},
		now:    time.Now,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(m.generation), m.loadResources(m.resourcesGen, m.imagesGen, m.networksGen, m.volumesGen), m.checkDockerHubLogin())
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
		if m.advanced.stage != advancedClosed {
			return m.updateAdvanced(msg)
		}
		if m.action.stage == actionConfirm {
			return m.updateConfirmation(msg)
		}
		if m.action.stage == actionMenu {
			return m.updateActionMenu(msg)
		}
		if key.Matches(msg, m.keys.advanced) && m.canOpenAdvanced() {
			m.advanced = advancedState{stage: advancedMenu}
			return m, nil
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
			if m.showHelp {
				return m, m.checkDockerHubLogin()
			}
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
		case key.Matches(msg, m.keys.selectAll):
			if m.editing {
				m.toggleAllContainerSelection()
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
			if !m.editing && m.selectedID != "" {
				m.action = actionState{stage: actionMenu, targets: m.containerTargetsForIDs([]string{m.selectedID})}
			}
			return m, nil
		case key.Matches(msg, m.keys.details):
			if !m.editing {
				return m.openDetails()
			}
			return m, nil
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
			if !sameRunningImageReferences(m.snapshot, msg.snapshot) {
				m.cancelImageUpdateScan()
				m.containerUpdates = make(map[string]domain.UpdateStatus)
			}
			m.snapshot = m.service.Sort(msg.snapshot, m.sortMode)
			m.prunePendingRecreates()
			m.applyContainerUpdateStatuses()
			m.snapshot = m.service.DecorateComposeSnapshot(m.snapshot)
			m.stacks = m.service.RebuildStacks(m.snapshot)
			m.stacksLoading, m.stacksLoaded, m.stacksErr = false, true, nil
			m.syncStackSelection()
		}
		m.err = msg.err
		m = m.syncSelection()
		return m, tea.Batch(scheduleRefresh(), m.startImageUpdateScan())
	case refreshMsg:
		if m.loading || m.refreshing {
			return m, nil
		}
		m.generation++
		m.refreshing = true
		return m, m.load(m.generation)
	case actionFinishedMsg:
		resource := msg.resource
		action := msg.action
		targets := msg.targets
		if resource == actionImages || isContainerUpdateAction(action) {
			m.cancelImageUpdateScan()
		}
		if resource == actionImages && action == application.ActionPull {
			m.markPulledImages(targets, msg.results)
		}
		if resource == actionImages && action == application.ActionRecreate {
			m.clearRecreatedImages(targets, msg.results)
		}
		if isContainerUpdateAction(action) {
			m.reconcileContainerUpdateResults(action, targets, msg.results)
		}
		m.action = actionState{}
		m.showActionResult(msg.results)
		expireNotice := m.expireActionNotice()
		if resource == actionImages {
			m.imageEditing = false
			m.clearImageSelection()
			m.imagesGen++
			m.imagesLoaded = false
			m.imagesLoading = true
			m.imagesErr = nil
			m.generation++
			m.refreshing = true
			return m, tea.Batch(m.load(m.generation), m.loadImages(m.imagesGen), expireNotice)
		}
		if resource == actionNetworks {
			m.networkEditing = false
			m.clearNetworkSelection()
			m.networksGen++
			m.networksLoaded = false
			m.networksLoading = true
			m.networksErr = nil
			return m, tea.Batch(m.loadNetworks(m.networksGen), expireNotice)
		}
		if resource == actionVolumes {
			m.volumeEditing = false
			m.clearVolumeSelection()
			m.volumesGen++
			m.volumesLoaded = false
			m.volumesLoading = true
			m.volumesErr = nil
			return m, tea.Batch(m.loadVolumes(m.volumesGen), expireNotice)
		}
		if resource == actionStacks {
			m.stackEditing = false
			m.clearStackSelection()
			m.generation++
			m.refreshing = true
			m.stacksGen++
			m.stacksLoaded, m.stacksLoading, m.stacksErr = false, true, nil
			if isContainerUpdateAction(action) {
				m.imagesGen++
				m.imagesLoaded, m.imagesLoading, m.imagesErr = false, true, nil
				return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen), m.loadImages(m.imagesGen), expireNotice)
			}
			return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen), expireNotice)
		}
		if resource == actionStackContainers {
			m.stackContainerEditing = false
			m.clearStackContainerSelection()
			m.generation++
			m.refreshing = true
			m.stacksGen++
			m.stacksLoaded, m.stacksLoading, m.stacksErr = false, true, nil
			if isContainerUpdateAction(action) {
				m.imagesGen++
				m.imagesLoaded, m.imagesLoading, m.imagesErr = false, true, nil
				return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen), m.loadImages(m.imagesGen), expireNotice)
			}
			return m, tea.Batch(m.load(m.generation), m.loadStacks(m.stacksGen), expireNotice)
		}
		m.editing = false
		m.clearSelection()
		m.generation++
		if isContainerUpdateAction(action) {
			m.imagesGen++
			m.imagesLoaded = false
			m.imagesLoading = true
			m.imagesErr = nil
			return m, tea.Batch(m.load(m.generation), m.loadImages(m.imagesGen), expireNotice)
		}
		return m, tea.Batch(m.load(m.generation), expireNotice)
	case actionNoticeExpiredMsg:
		if msg.generation != m.noticeGeneration {
			return m, nil
		}
		if m.action.running || m.action.stage == actionConfirm {
			return m, nil
		}
		m.notice = ""
		return m, nil
	case resourcesLoadedMsg:
		if msg.generation != 0 && msg.generation != m.resourcesGen {
			return m, nil
		}
		m.resourcesLoading = false
		// Stacks are reconstructed exclusively from Container snapshots.
		if m.imagesGen == msg.imagesGen {
			m.reconcileImages(msg.resources.Images, resourceLoadError(msg.err, msg.resources.ImagesErr))
		}
		if m.networksGen == msg.networksGen {
			m.reconcileNetworks(msg.resources.Networks, resourceLoadError(msg.err, msg.resources.NetworksErr))
		}
		if m.volumesGen == msg.volumesGen {
			m.reconcileVolumes(msg.resources.Volumes, resourceLoadError(msg.err, msg.resources.VolumesErr))
		}
		return m, tea.Batch(scheduleResourceRefresh(), m.startImageUpdateScan())
	case imageUpdatesLoadedMsg:
		if msg.generation != m.updatesGeneration {
			return m, nil
		}
		m.updatesRunning = false
		m.updatesCancel = nil
		for id := range m.updatesChecking {
			m.setImageUpdate(id, domain.UpdateUnknown)
		}
		if m.containerUpdates == nil {
			m.containerUpdates = make(map[string]domain.UpdateStatus)
		}
		imageStatuses := make(map[string]domain.UpdateStatus)
		for _, update := range msg.updates {
			if update.Reason == application.DockerHubLoginRequiredReason {
				m.dockerHubLoginChecked = true
				m.dockerHubLoginConfigured = false
			}
			if update.Status == domain.UpdatePulledPendingRecreate {
				m.recordPendingRecreate(update)
			}
			if update.ContainerID != "" {
				m.containerUpdates[update.ContainerID] = update.Status
			}
			imageStatuses[update.ImageID] = aggregateUpdateStatus(imageStatuses[update.ImageID], update.Status)
		}
		for imageID, status := range imageStatuses {
			if !m.imagePendingRecreate(imageID) {
				m.setImageUpdate(imageID, status)
			}
		}
		m.updatesChecking = make(map[string]domain.UpdateStatus)
		m.applyPendingImageStatuses()
		m.applyContainerUpdateStatuses()
		m.snapshot = m.service.DecorateComposeSnapshot(m.snapshot)
		m.stacks = m.service.RebuildStacks(m.snapshot)
		return m, scheduleImageUpdateRefresh(msg.generation, m.updates.Interval())
	case imageUpdateRefreshMsg:
		if msg.generation != m.updatesGeneration {
			return m, nil
		}
		m.updatesStarted = false
		return m, m.startImageUpdateScan()
	case dockerHubLoginLoadedMsg:
		wasConfigured := m.dockerHubLoginConfigured
		m.dockerHubLoginChecked = true
		m.dockerHubLoginConfigured = msg.configured
		if msg.configured && !wasConfigured {
			m.updates.Invalidate(m.dockerHubReferences()...)
			m.cancelImageUpdateScan()
			return m, m.startImageUpdateScan()
		}
		return m, nil
	case resourceRefreshMsg:
		if m.resourcesLoading {
			return m, nil
		}
		m.resourcesLoading = true
		m.resourcesGen++
		return m, m.loadResources(m.resourcesGen, m.imagesGen, m.networksGen, m.volumesGen)
	case imagesLoadedMsg:
		if msg.generation != m.imagesGen {
			return m, nil
		}
		m.imagesLoading = false
		m.imagesLoaded = true
		m.images = preserveImageUpdates(m.images, msg.images)
		m.imagesErr = msg.err
		m.applyPendingImageStatuses()
		m.applyContainerUpdateStatuses()
		m.snapshot = m.service.DecorateComposeSnapshot(m.snapshot)
		m.stacks = m.service.RebuildStacks(m.snapshot)
		m.syncImageSelection()
		return m, m.startImageUpdateScan()
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
	case advancedFinishedMsg:
		m.advanced.stage = advancedResult
		m.advanced.result = msg.result
		m.advanced.input = ""
		m.generation++
		m.refreshing = true
		m.resourcesGen++
		m.resourcesLoading = true
		m.imagesGen++
		m.networksGen++
		m.volumesGen++
		return m, tea.Batch(
			m.load(m.generation),
			m.loadResources(m.resourcesGen, m.imagesGen, m.networksGen, m.volumesGen),
		)
	}

	return m, nil
}

func (m Model) View() string {
	layout := sharedui.ResolveLayout(m.width, m.height)
	views := []sharedui.View{
		m.containersView(layout),
		m.stacksView(layout),
		m.imagesView(layout),
		m.networksView(layout),
		m.volumesView(layout),
	}
	if m.advanced.stage != advancedClosed && m.active >= 0 && m.active < len(views) {
		views[m.active] = m.advancedView(views[m.active].Title, layout)
	}
	shell := sharedui.NewShell(sharedui.ShellOptions{
		Title:        "dtop",
		Localizer:    m.localizer,
		Subtitle:     m.headerSummary(),
		ActiveView:   m.active,
		Footer:       m.footer(layout),
		FooterNotice: m.dockerHubFooterNotice(),
		AccentColor:  m.accentColor,
		Banner:       m.confirmationBanner(),
		BannerColor:  "33",
		Views:        views,
	})

	if m.width > 0 || m.height > 0 {
		updated, _ := shell.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		shell = updated
	}

	return shell.View()
}

func (m Model) imagesView(layout sharedui.Layout) sharedui.View {
	title := m.localizer.Text(i18n.MessageTabImages)
	if m.action.resource == actionImages && m.action.stage == actionMenu {
		return m.imagesActionView()
	}
	if m.imageDetailOpen {
		return m.imageDetailsView()
	}
	if m.imagesLoading || !m.imagesLoaded {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageImagesLoading)}
	}
	if m.imagesErr != nil {
		if len(m.images) > 0 {
			return sharedui.View{Title: title, Status: sharedui.StatusWarning, Summary: m.localizer.Text(i18n.MessageImagesPartial) + m.imagesErr.Error(), Sections: []sharedui.Section{{Body: renderImagesLocalized(m.images, m.selectedImageID, m.selectedImages, m.imageEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}}}
		}
		return sharedui.View{
			Title: title, Status: sharedui.StatusError, Summary: m.imagesErr.Error(),
			Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageCommonRetry)}},
		}
	}
	if len(m.images) == 0 {
		return sharedui.View{Title: title, Status: sharedui.StatusEmpty, Summary: m.localizer.Text(i18n.MessageImagesEmpty)}
	}
	return sharedui.View{
		Title: title, Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{{Body: renderImagesLocalized(m.images, m.selectedImageID, m.selectedImages, m.imageEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}},
	}
}

func (m Model) imageDetailsView() sharedui.View {
	title := m.localizer.Text(i18n.MessageImageDetailsTitle)
	if m.imageDetailLoading {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageImageDetailsLoad)}
	}
	if m.imageDetailErr != nil {
		return sharedui.View{
			Title: title, Status: sharedui.StatusError, Summary: m.imageDetailErr.Error(),
			Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}},
		}
	}
	details := m.imageDetails
	tags := m.localizer.Text(i18n.MessageCommonUntagged)
	if len(details.Tags) > 0 {
		tags = strings.Join(details.Tags, "\n")
	}
	digests := "-"
	if len(details.Digests) > 0 {
		digests = strings.Join(details.Digests, "\n")
	}
	return sharedui.View{
		Title: title, Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{
			{Title: shortContainerID(details.ID), Body: strings.Join([]string{m.localizer.Text(i18n.MessageImageDetailsSize, formatBytes(details.Size)), m.localizer.Text(i18n.MessageImageDetailsCreate, formatImageAge(details.Created, m.now())), m.localizer.Text(i18n.MessageImageDetailsPlat, details.OS+"/"+details.Architecture)}, "\n")},
			{Title: m.localizer.Text(i18n.MessageSectionTags), Body: tags},
			{Title: m.localizer.Text(i18n.MessageSectionDigests), Body: digests},
			{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)},
		},
	}
}

func (m Model) networksView(layout sharedui.Layout) sharedui.View {
	title := m.localizer.Text(i18n.MessageTabNetworks)
	if m.networkDetailOpen {
		return m.networkDetailsView()
	}
	if m.networksLoading || !m.networksLoaded {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageNetworksLoading)}
	}
	if m.networksErr != nil {
		if len(m.networks) > 0 {
			return sharedui.View{Title: title, Status: sharedui.StatusWarning, Summary: m.localizer.Text(i18n.MessageNetworksPartial) + m.networksErr.Error(), Sections: []sharedui.Section{{Body: renderNetworksLocalized(m.networks, m.selectedNetworkID, m.selectedNetworks, m.networkEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}}}
		}
		return sharedui.View{Title: title, Status: sharedui.StatusError, Summary: m.networksErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageCommonRetry)}}}
	}
	if len(m.networks) == 0 {
		return sharedui.View{Title: title, Status: sharedui.StatusEmpty, Summary: m.localizer.Text(i18n.MessageNetworksEmpty)}
	}
	if m.action.resource == actionNetworks && m.action.stage == actionMenu {
		return m.actionView()
	}
	return sharedui.View{Title: title, Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Body: renderNetworksLocalized(m.networks, m.selectedNetworkID, m.selectedNetworks, m.networkEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}}}
}

func (m Model) networkDetailsView() sharedui.View {
	title := m.localizer.Text(i18n.MessageNetworkDetailsTitle)
	if m.networkDetailLoading {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageNetworkDetailsLoad)}
	}
	if m.networkDetailErr != nil {
		return sharedui.View{Title: title, Status: sharedui.StatusError, Summary: m.networkDetailErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}}}
	}
	details := m.networkDetails
	containers := "-"
	if len(details.Containers) > 0 {
		containers = strings.Join(details.Containers, "\n")
	}
	return sharedui.View{Title: title, Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Title: details.Name, Body: strings.Join([]string{m.localizer.Text(i18n.MessageDetailsID, shortContainerID(details.ID)), m.localizer.Text(i18n.MessageDetailsDriver, details.Driver), m.localizer.Text(i18n.MessageDetailsScope, details.Scope), m.localizer.Text(i18n.MessageDetailsCreated, formatImageAge(details.Created, m.now())), m.localizer.Text(i18n.MessageDetailsInternal, yesNoLocalized(details.Internal, m.localizer)), m.localizer.Text(i18n.MessageDetailsAttachable, yesNoLocalized(details.Attachable, m.localizer))}, "\n")}, {Title: m.localizer.Text(i18n.MessageSectionContainers), Body: containers}, {Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}}}
}

func (m Model) volumesView(layout sharedui.Layout) sharedui.View {
	title := m.localizer.Text(i18n.MessageTabVolumes)
	if m.volumeDetailOpen {
		return m.volumeDetailsView()
	}
	if m.volumesLoading || !m.volumesLoaded {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageVolumesLoading)}
	}
	if m.volumesErr != nil {
		if len(m.volumes) > 0 {
			return sharedui.View{Title: title, Status: sharedui.StatusWarning, Summary: m.localizer.Text(i18n.MessageVolumesPartial) + m.volumesErr.Error(), Sections: []sharedui.Section{{Body: renderVolumesLocalized(m.volumes, m.selectedVolumeName, m.selectedVolumes, m.volumeEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}}}
		}
		return sharedui.View{Title: title, Status: sharedui.StatusError, Summary: m.volumesErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageCommonRetry)}}}
	}
	if len(m.volumes) == 0 {
		return sharedui.View{Title: title, Status: sharedui.StatusEmpty, Summary: m.localizer.Text(i18n.MessageVolumesEmpty)}
	}
	if m.action.resource == actionVolumes && m.action.stage == actionMenu {
		return m.actionView()
	}
	return sharedui.View{Title: title, Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Body: renderVolumesLocalized(m.volumes, m.selectedVolumeName, m.selectedVolumes, m.volumeEditing, layout, m.now(), m.accentColor, m.focusColor, m.localizer)}}}
}

func (m Model) volumeDetailsView() sharedui.View {
	title := m.localizer.Text(i18n.MessageVolumeDetailsTitle)
	if m.volumeDetailLoading {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageVolumeDetailsLoad)}
	}
	if m.volumeDetailErr != nil {
		return sharedui.View{Title: title, Status: sharedui.StatusError, Summary: m.volumeDetailErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}}}
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
	return sharedui.View{Title: title, Status: sharedui.StatusReady, HideStatus: true, Sections: []sharedui.Section{{Title: details.Name, Body: strings.Join([]string{m.localizer.Text(i18n.MessageDetailsDriver, details.Driver), m.localizer.Text(i18n.MessageDetailsScope, details.Scope), m.localizer.Text(i18n.MessageDetailsMountpoint, details.Mountpoint), m.localizer.Text(i18n.MessageDetailsCreated, formatImageAge(details.Created, m.now()))}, "\n")}, {Title: m.localizer.Text(i18n.MessageSectionOptions), Body: options}, {Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}}}
}

func (m Model) load(generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()

		snapshot, err := m.service.Load(ctx)
		return loadedMsg{snapshot: snapshot, err: err, generation: generation}
	}
}

func (m Model) loadResources(generation, imagesGen, networksGen, volumesGen uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()

		resources, err := m.service.LoadResources(ctx)
		return resourcesLoadedMsg{resources: resources, err: err, generation: generation, imagesGen: imagesGen, networksGen: networksGen, volumesGen: volumesGen}
	}
}

func (m Model) checkDockerHubLogin() tea.Cmd {
	if m.dockerHubLoginCheck == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		defer cancel()
		return dockerHubLoginLoadedMsg{configured: m.dockerHubLoginCheck(ctx)}
	}
}

func (m Model) dockerHubReferences() []string {
	references := make([]string, 0)
	for _, container := range m.snapshot.Containers {
		if normalized, ok := application.NormalizeImageReference(container.Image); ok && strings.HasPrefix(normalized, "docker.io/") {
			references = append(references, container.Image)
		}
	}
	return references
}

func resourceLoadError(loadErr, sourceErr error) error {
	if loadErr != nil {
		return loadErr
	}
	return sourceErr
}

func (m *Model) reconcileImages(images []domain.Image, err error) {
	m.imagesLoading, m.imagesLoaded, m.imagesErr = false, true, err
	if err != nil {
		return
	}
	changed := !sameImageDigests(m.images, images)
	if changed {
		m.cancelImageUpdateScan()
	} else {
		previous := make(map[string]domain.UpdateStatus, len(m.images))
		for _, image := range m.images {
			previous[image.ID] = image.Update
		}
		for index := range images {
			images[index].Update = previous[images[index].ID]
		}
	}
	wasOpen, selected := m.imageDetailOpen, m.selectedImageID
	m.images = images
	for index := range m.images {
		if m.images[index].Update == "" {
			m.images[index].Update = domain.UpdateUnknown
		}
	}
	m.applyPendingImageStatuses()
	m.syncImageSelection()
	if wasOpen && selected != m.selectedImageID {
		m.imageDetailOpen = false
	}
}

func (m *Model) startImageUpdateScan() tea.Cmd {
	if m.updates == nil || !m.updates.Enabled() || m.updatesStarted || m.updatesRunning || m.loading || m.err != nil || m.resourcesLoading || m.imagesErr != nil || !m.imagesLoaded {
		return nil
	}
	m.updatesStarted, m.updatesRunning = true, true
	m.updatesGeneration++
	generation := m.updatesGeneration
	eligibleImageIDs := make(map[string]struct{})
	scanSnapshot := m.snapshot
	scanSnapshot.Containers = make([]domain.Container, 0, len(m.snapshot.Containers))
	for _, container := range m.snapshot.Containers {
		id := normalizedImageID(container.ImageID)
		if container.State != "running" || id == "" || m.containerPendingRecreate(container.ID) {
			continue
		}
		if _, ok := application.NormalizeImageReference(container.Image); ok {
			eligibleImageIDs[id] = struct{}{}
			scanSnapshot.Containers = append(scanSnapshot.Containers, container)
		} else if !m.imagePendingRecreate(id) {
			m.setImageUpdate(id, domain.UpdateUnknown)
		}
	}
	m.updatesChecking = make(map[string]domain.UpdateStatus, len(eligibleImageIDs))
	for index := range m.images {
		id := normalizedImageID(m.images[index].ID)
		if _, found := eligibleImageIDs[id]; found && !m.imagePendingRecreate(id) {
			m.updatesChecking[id] = m.images[index].Update
			m.images[index].Update = domain.UpdateChecking
		}
	}
	images := append([]domain.Image(nil), m.images...)
	ctx, cancel := context.WithCancel(m.ctx)
	m.updatesCancel = cancel
	return func() tea.Msg {
		return imageUpdatesLoadedMsg{updates: m.updates.Scan(ctx, scanSnapshot, images), generation: generation}
	}
}

func (m *Model) cancelImageUpdateScan() {
	if m.updatesCancel != nil {
		m.updatesCancel()
	}
	m.updatesCancel = nil
	for id, previous := range m.updatesChecking {
		for index := range m.images {
			if normalizedImageID(m.images[index].ID) == id && m.images[index].Update == domain.UpdateChecking {
				m.images[index].Update = previous
			}
		}
	}
	m.updatesChecking = make(map[string]domain.UpdateStatus)
	m.updatesRunning = false
	m.updatesStarted = false
	m.updatesGeneration++
}

func sameRunningImageReferences(left, right domain.Snapshot) bool {
	references := func(snapshot domain.Snapshot) map[string]string {
		result := make(map[string]string)
		for _, container := range snapshot.Containers {
			if container.State == "running" {
				result[container.ID] = container.ImageID + "|" + container.Image
			}
		}
		return result
	}
	return reflect.DeepEqual(references(left), references(right))
}

func sameImageDigests(left, right []domain.Image) bool {
	digests := func(images []domain.Image) map[string]string {
		result := make(map[string]string, len(images))
		for _, image := range images {
			result[image.ID] = strings.Join(image.RepoDigests, "|")
		}
		return result
	}
	return reflect.DeepEqual(digests(left), digests(right))
}

func scheduleImageUpdateRefresh(generation uint64, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg { return imageUpdateRefreshMsg{generation: generation} })
}

func (m *Model) reconcileNetworks(networks []domain.Network, err error) {
	m.networksLoading, m.networksLoaded, m.networksErr = false, true, err
	if err != nil {
		return
	}
	wasOpen, selected := m.networkDetailOpen, m.selectedNetworkID
	m.networks = networks
	m.syncNetworkSelection()
	if wasOpen && selected != m.selectedNetworkID {
		m.networkDetailOpen = false
	}
}

func (m *Model) reconcileVolumes(volumes []domain.Volume, err error) {
	m.volumesLoading, m.volumesLoaded, m.volumesErr = false, true, err
	if err != nil {
		return
	}
	wasOpen, selected := m.volumeDetailOpen, m.selectedVolumeName
	m.volumes = volumes
	m.syncVolumeSelection()
	if wasOpen && selected != m.selectedVolumeName {
		m.volumeDetailOpen = false
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
	case key.Matches(msg, m.keys.selectAll):
		if m.stackContainerEditing {
			m.toggleAllStackContainerSelection()
		} else if m.stackEditing {
			m.toggleAllStackSelection()
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
	case key.Matches(msg, m.keys.selectAll):
		if m.imageEditing && !m.imageDetailOpen {
			m.toggleAllImageSelection()
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
		m.action = actionState{stage: actionMenu, resource: actionImages, targets: m.selectedImageTargetsForIDs([]string{m.selectedImageID})}
		return m, nil
	case key.Matches(msg, m.keys.details):
		if m.imageDetailOpen || m.imageEditing || m.selectedImageID == "" {
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
	case key.Matches(msg, m.keys.selectAll):
		if m.networkEditing && !m.networkDetailOpen {
			m.toggleAllNetworkSelection()
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
		m.action = actionState{stage: actionMenu, resource: actionNetworks, targets: m.networkTargetsForIDs([]string{m.selectedNetworkID})}
	case key.Matches(msg, m.keys.details):
		if m.networkDetailOpen || m.networkEditing || m.selectedNetworkID == "" {
			return m, nil
		}
		m.networkDetailOpen, m.networkDetailLoading, m.networkDetailErr, m.networkDetails = true, true, nil, domain.NetworkDetails{}
		id := m.selectedNetworkID
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
			defer cancel()
			details, err := m.service.NetworkDetails(ctx, id)
			return networkDetailsLoadedMsg{details: details, err: err}
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
	case key.Matches(msg, m.keys.selectAll):
		if m.volumeEditing && !m.volumeDetailOpen {
			m.toggleAllVolumeSelection()
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
		m.action = actionState{stage: actionMenu, resource: actionVolumes, targets: m.volumeTargetsForNames([]string{m.selectedVolumeName})}
	case key.Matches(msg, m.keys.details):
		if m.volumeDetailOpen || m.volumeEditing || m.selectedVolumeName == "" {
			return m, nil
		}
		m.volumeDetailOpen, m.volumeDetailLoading, m.volumeDetailErr, m.volumeDetails = true, true, nil, domain.VolumeDetails{}
		name := m.selectedVolumeName
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
			defer cancel()
			details, err := m.service.VolumeDetails(ctx, name)
			return volumeDetailsLoadedMsg{details: details, err: err}
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
	available := make(map[string]struct{}, len(m.images))
	for _, image := range m.images {
		available[image.ID] = struct{}{}
	}
	for id := range m.selectedImages {
		if _, found := available[id]; !found {
			delete(m.selectedImages, id)
		}
	}
	if _, found := available[m.selectedImageID]; found {
		return
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
func (m *Model) toggleAllStackSelection() {
	ids := make([]string, 0, len(m.stacks))
	for _, stack := range m.stacks {
		ids = append(ids, stack.Name)
	}
	toggleAll(&m.selectedStacks, ids)
}
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
			stackCopy := stack
			targets = append(targets, actionTarget{ID: stack.Name, Name: stack.Name, State: stack.State, Update: stackUpdateStatus(stack), UpdatePending: stack.UpdatePending, UpdateUnknown: stack.UpdateUnknown, Unavailable: stack.DownUnavailableReason(), Stack: &stackCopy})
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
func (m *Model) toggleAllStackContainerSelection() {
	ids := make([]string, 0)
	if stack := m.selectedStack(); stack != nil && stack.Name == m.expandedStackName {
		for _, container := range stack.ContainerItems {
			ids = append(ids, container.ID)
		}
	}
	toggleAll(&m.selectedStackContainers, ids)
}
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
				stackCopy := *stack
				targets = append(targets, actionTarget{ID: container.ID, Name: container.Name, State: container.State, ImageID: container.ImageID, Update: container.Update, Stack: &stackCopy, Service: container.ComposeService})
			}
		}
	}
	return targets
}

func (m *Model) clearImageSelection() {
	m.selectedImages = make(map[string]struct{})
}

func (m *Model) toggleAllImageSelection() {
	ids := make([]string, 0, len(m.images))
	for _, image := range m.images {
		ids = append(ids, image.ID)
	}
	toggleAll(&m.selectedImages, ids)
}

func (m *Model) clearNetworkSelection() {
	m.selectedNetworks = make(map[string]struct{})
}

func (m *Model) toggleAllNetworkSelection() {
	ids := make([]string, 0, len(m.networks))
	for _, network := range m.networks {
		ids = append(ids, network.ID)
	}
	toggleAll(&m.selectedNetworks, ids)
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
	ids := make([]string, 0, len(m.selectedNetworks))
	for id := range m.selectedNetworks {
		ids = append(ids, id)
	}
	return m.networkTargetsForIDs(ids)
}

func (m Model) networkTargetsForIDs(ids []string) []actionTarget {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	targets := make([]actionTarget, 0, len(ids))
	for _, network := range m.networks {
		if _, found := selected[network.ID]; found {
			targets = append(targets, actionTarget{ID: network.ID, Name: network.Name})
		}
	}
	return targets
}

func (m *Model) clearVolumeSelection() {
	m.selectedVolumes = make(map[string]struct{})
}

func (m *Model) toggleAllVolumeSelection() {
	ids := make([]string, 0, len(m.volumes))
	for _, volume := range m.volumes {
		ids = append(ids, volume.Name)
	}
	toggleAll(&m.selectedVolumes, ids)
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
	names := make([]string, 0, len(m.selectedVolumes))
	for name := range m.selectedVolumes {
		names = append(names, name)
	}
	return m.volumeTargetsForNames(names)
}

func (m Model) volumeTargetsForNames(names []string) []actionTarget {
	selected := make(map[string]struct{}, len(names))
	for _, name := range names {
		selected[name] = struct{}{}
	}
	targets := make([]actionTarget, 0, len(names))
	for _, volume := range m.volumes {
		if _, found := selected[volume.Name]; found {
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
	ids := make([]string, 0, len(m.selectedImages))
	for _, image := range m.images {
		if _, selected := m.selectedImages[image.ID]; selected {
			ids = append(ids, image.ID)
		}
	}
	return m.selectedImageTargetsForIDs(ids)
}

func (m Model) selectedImageTargetsForIDs(ids []string) []actionTarget {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	targets := make([]actionTarget, 0, len(m.selectedImages))
	for _, image := range m.images {
		if _, found := selected[image.ID]; found {
			target := actionTarget{ID: image.ID, Name: image.Name}
			if image.Update == domain.UpdateAvailable {
				for _, container := range m.snapshot.Containers {
					if container.State == "running" && strings.TrimPrefix(strings.ToLower(container.ImageID), "sha256:") == strings.TrimPrefix(strings.ToLower(image.ID), "sha256:") {
						target.PullRefs = append(target.PullRefs, container.Image)
					}
				}
			}
			if image.Update == domain.UpdatePulledPendingRecreate {
				for _, pending := range m.pendingRecreates {
					if !pending.Compose && normalizedImageID(pending.ImageID) == normalizedImageID(image.ID) {
						target.Recreate = append(target.Recreate, application.RecreateTarget{ID: pending.ContainerID, Reference: pending.Reference})
					}
				}
			}
			targets = append(targets, target)
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

func (m Model) canOpenAdvanced() bool {
	return m.action.stage == actionNone && !m.action.running && m.panel == panelContainers &&
		!m.showHelp && !m.editing && !m.stackEditing && !m.stackContainerEditing &&
		!m.imageEditing && !m.networkEditing && !m.volumeEditing &&
		!m.imageDetailOpen && !m.networkDetailOpen && !m.volumeDetailOpen && !m.shellActive
}

func (m Model) updateAdvanced(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.advanced.stage {
	case advancedMenu:
		switch {
		case key.Matches(msg, m.keys.back), key.Matches(msg, m.keys.advanced):
			m.advanced = advancedState{}
		case key.Matches(msg, m.keys.up):
			if m.advanced.index > 0 {
				m.advanced.index--
			}
		case key.Matches(msg, m.keys.down):
			if m.advanced.index < len(advancedPruneChoices())-1 {
				m.advanced.index++
			}
		case key.Matches(msg, m.keys.confirm):
			if _, ok := m.selectedPruneKind(); !ok {
				m.advanced = advancedState{}
			} else {
				m.advanced.stage = advancedConfirm
				m.advanced.input = ""
			}
		}
	case advancedConfirm:
		if key.Matches(msg, m.keys.back) {
			m.advanced.stage = advancedMenu
			m.advanced.input = ""
			return m, nil
		}
		if msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
			input := []rune(m.advanced.input)
			if len(input) > 0 {
				m.advanced.input = string(input[:len(input)-1])
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.confirm) {
			if m.advanced.input == "prune" {
				kind, _ := m.selectedPruneKind()
				m.advanced.stage = advancedRunning
				return m, m.runPrune(kind)
			}
			return m, nil
		}
		if msg.Type == tea.KeyRunes {
			for _, value := range msg.Runes {
				if value >= ' ' && value != '\x7f' && len([]rune(m.advanced.input)) < 32 {
					m.advanced.input += string(value)
				}
			}
		}
	case advancedResult:
		if key.Matches(msg, m.keys.back) || key.Matches(msg, m.keys.confirm) || key.Matches(msg, m.keys.advanced) {
			m.advanced = advancedState{}
		}
	}
	return m, nil
}

type advancedPruneChoice struct {
	kind    application.PruneKind
	message string
}

func advancedPruneChoices() []advancedPruneChoice {
	return []advancedPruneChoice{
		{kind: application.PruneContainers, message: i18n.MessageAdvancedDeleteContainers},
		{kind: application.PruneImages, message: i18n.MessageAdvancedDeleteImages},
		{kind: application.PruneNetworks, message: i18n.MessageAdvancedDeleteNetworks},
		{kind: application.PruneVolumes, message: i18n.MessageAdvancedDeleteVolumes},
		{kind: application.PruneSystem, message: i18n.MessageAdvancedDeleteSystem},
		{message: i18n.MessageActionCancel},
	}
}

func (m Model) selectedPruneKind() (application.PruneKind, bool) {
	choices := advancedPruneChoices()
	if m.advanced.index < 0 || m.advanced.index >= len(choices) || choices[m.advanced.index].kind == "" {
		return "", false
	}
	return choices[m.advanced.index].kind, true
}

func (m Model) runPrune(kind application.PruneKind) tea.Cmd {
	return func() tea.Msg {
		return advancedFinishedMsg{result: m.service.Prune(m.ctx, kind)}
	}
}

func (m Model) advancedView(title string, layout sharedui.Layout) sharedui.View {
	if m.advanced.stage == advancedRunning {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageAdvancedRunning), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageAdvancedCommandTitle), Body: m.advancedCommandLine()}}}
	}
	if m.advanced.stage == advancedResult {
		body := m.advanced.result.Output
		status := sharedui.StatusReady
		if m.advanced.result.Err != nil {
			status = sharedui.StatusError
			if body != "" {
				body = m.advanced.result.Err.Error() + "\n\n" + body
			} else {
				body = m.advanced.result.Err.Error()
			}
		}
		if body == "" {
			body = m.localizer.Text(i18n.MessageAdvancedCompleted)
		}
		return sharedui.View{Title: title, Status: status, HideStatus: status == sharedui.StatusReady, Sections: []sharedui.Section{
			{Title: m.localizer.Text(i18n.MessageAdvancedResultTitle), Body: body},
			{Title: m.localizer.Text(i18n.MessageAdvancedCommandTitle), Body: m.advancedResultCommandLine()},
			{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageAdvancedResultControls)},
		}}
	}

	choices := advancedPruneChoices()
	lines := make([]string, 0, len(choices))
	width := layout.ContentWidth
	if width < 20 {
		width = 20
	}
	for index, choice := range choices {
		label := m.localizer.Text(choice.message)
		line := "  " + label
		if index == m.advanced.index {
			line = focusedMenuRow("> "+label, width, m.focusColor, m.accentColor)
		}
		lines = append(lines, line)
	}
	return sharedui.View{Title: title, Status: sharedui.StatusWarning, HideStatus: true, Sections: []sharedui.Section{
		{Title: m.localizer.Text(i18n.MessageAdvancedTitle), Body: strings.Join(lines, "\n")},
		{Title: m.localizer.Text(i18n.MessageAdvancedCommandTitle), Body: m.advancedCommandLine()},
		{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageAdvancedControls)},
	}}
}

func (m Model) advancedCommandLine() string {
	kind, ok := m.selectedPruneKind()
	if !ok {
		return m.localizer.Text(i18n.MessageAdvancedNoCommand)
	}
	return m.localizer.Text(i18n.MessageAdvancedCommand, application.PruneCommandText(kind))
}

func (m Model) advancedResultCommandLine() string {
	if len(m.advanced.result.Command) == 0 {
		return m.localizer.Text(i18n.MessageAdvancedNoCommand)
	}
	return m.localizer.Text(i18n.MessageAdvancedCommand, strings.Join(m.advanced.result.Command, " "))
}

func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(now time.Time) tea.Msg {
		return refreshMsg(now)
	})
}

func scheduleResourceRefresh() tea.Cmd {
	return tea.Tick(resourceRefreshInterval, func(now time.Time) tea.Msg {
		return resourceRefreshMsg(now)
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
		} else if m.selectedAction() == application.ActionPull {
			m.action.running = true
			return m, m.runAction(application.ActionPull)
		} else {
			m.action.stage = actionConfirm
			m.notice = ""
		}
	}

	return m, nil
}

func (m Model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.back) {
		m.action = actionState{}
		m.notice = ""
		m.noticeGeneration++
		return m, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y' || msg.Runes[0] == 's' || msg.Runes[0] == 'S') {
		m.action.running = true
		m.noticeGeneration++
		return m, m.runAction(m.selectedAction())
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'n' || msg.Runes[0] == 'N') {
		m.action = actionState{}
		m.notice = ""
		m.noticeGeneration++
	}

	return m, nil
}

func (m Model) runAction(action application.Action) tea.Cmd {
	targets := append([]actionTarget(nil), m.action.targets...)
	resource := m.action.resource
	return func() tea.Msg {
		ctx := m.ctx
		if !isContainerUpdateAction(action) {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(m.ctx, 45*time.Second)
			defer cancel()
		}

		ids := make([]string, 0, len(targets))
		for _, target := range targets {
			ids = append(ids, target.ID)
		}
		finished := func(results []application.ActionResult) tea.Msg {
			return actionFinishedMsg{results: results, resource: resource, action: action, targets: targets}
		}
		if resource == actionImages {
			if action == application.ActionPull {
				references := make([]string, 0, len(targets))
				for _, target := range targets {
					references = append(references, target.PullRefs...)
				}
				return finished(m.service.PullImages(ctx, references))
			}
			if action == application.ActionRecreate {
				recreate := make([]application.RecreateTarget, 0, len(targets))
				for _, target := range targets {
					recreate = append(recreate, target.Recreate...)
				}
				return finished(m.service.RecreateImageContainers(ctx, recreate))
			}
			return finished(m.service.RemoveImages(ctx, ids))
		}
		if resource == actionNetworks {
			return finished(m.service.RemoveNetworks(ctx, ids))
		}
		if resource == actionVolumes {
			return finished(m.service.RemoveVolumes(ctx, ids))
		}
		if resource == actionStacks {
			stacks := make([]domain.Stack, 0, len(targets))
			for _, target := range targets {
				if isContainerUpdateAction(action) && !targetUpdateEligible(action, target) {
					continue
				}
				for _, stack := range m.stacks {
					if stack.Name == target.ID {
						stacks = append(stacks, stack)
						break
					}
				}
			}
			return finished(m.service.ActStacks(ctx, action, stacks))
		}
		if resource == actionStackContainers && isContainerUpdateAction(action) {
			return finished(m.runComposeContainerUpdate(ctx, action, targets))
		}
		if resource == actionContainers && isContainerUpdateAction(action) {
			return finished(m.runContainerUpdate(ctx, action, targets))
		}
		return finished(m.service.Act(ctx, action, ids))
	}
}

func (m Model) actionMenuView() sharedui.View {
	choices := m.actionChoices()
	lines := make([]string, 0, len(choices))
	width := sharedui.ResolveLayout(m.width, m.height).ContentWidth
	if width < 20 {
		width = 20
	}
	for index, choice := range choices {
		line := "  " + actionLabelForResourceLocalized(choice, m.action.resource, m.localizer)
		if index == m.action.index {
			line = focusedMenuRow("> "+actionLabelForResourceLocalized(choice, m.action.resource, m.localizer), width, m.focusColor, m.accentColor)
		}
		lines = append(lines, line)
	}

	return sharedui.View{
		Title:      actionResourceLabelLocalized(m.action.resource, m.localizer),
		Status:     sharedui.StatusWarning,
		HideStatus: true,
		Sections: append([]sharedui.Section{
			{Title: m.localizer.Text(i18n.MessageActionSelected, m.actionResourceCount(len(m.action.targets))), Body: strings.Join(m.targetNames(m.action.targets), "\n")},
		}, m.actionMenuSections(lines)...),
	}
}

func (m Model) actionMenuSections(lines []string) []sharedui.Section {
	sections := []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionAction), Body: strings.Join(lines, "\n")}}
	if m.action.resource == actionContainers || m.action.resource == actionStackContainers || m.action.resource == actionStacks {
		eligible := 0
		for _, target := range m.action.targets {
			if targetUpdateEligible(m.selectedAction(), target) {
				eligible++
			}
		}
		if eligible > 0 && eligible < len(m.action.targets) {
			sections = append(sections, sharedui.Section{Title: m.localizer.Text(i18n.MessageSectionUpdateEligibility), Body: m.localizer.Text(i18n.MessageActionEligibility, eligible, len(m.action.targets)-eligible)})
		}
	}
	return append(sections, sharedui.Section{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageActionControls)})
}

func isContainerUpdateAction(action application.Action) bool {
	return action == application.ActionPull || action == application.ActionUpdate || action == application.ActionApply
}

func targetUpdateEligible(action application.Action, target actionTarget) bool {
	if target.Unavailable != "" {
		return false
	}
	if target.Stack != nil && target.Stack.DownUnavailableReason() != "" {
		return false
	}
	if target.Stack != nil && !target.Stack.Registered {
		return false
	}
	switch action {
	case application.ActionPull, application.ActionUpdate:
		return (target.Update == domain.UpdateAvailable || target.Stack != nil && (target.UpdatePending && target.UpdateUnknown || target.Update == domain.UpdatePulledPendingRecreate && !target.UpdatePending)) && (target.Stack != nil || len(target.Recreate) > 0)
	case application.ActionApply:
		if target.Stack != nil {
			return target.Update == domain.UpdatePulledPendingRecreate && target.UpdatePending
		}
		return target.Update == domain.UpdatePulledPendingRecreate && len(target.Recreate) > 0
	default:
		return false
	}
}

func (m Model) anyEligibleTarget(action application.Action) bool {
	for _, target := range m.action.targets {
		if targetUpdateEligible(action, target) {
			return true
		}
	}
	return false
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

func (m *Model) toggleAllContainerSelection() {
	ids := make([]string, 0, len(m.snapshot.Containers))
	for _, container := range m.snapshot.Containers {
		ids = append(ids, container.ID)
	}
	toggleAll(&m.selected, ids)
}

func toggleAll(selection *map[string]struct{}, ids []string) {
	if *selection == nil {
		*selection = make(map[string]struct{})
	}
	allSelected := len(ids) > 0
	for _, id := range ids {
		if _, selected := (*selection)[id]; !selected {
			allSelected = false
			break
		}
	}
	if allSelected {
		*selection = make(map[string]struct{})
		return
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	*selection = selected
}

func (m Model) selectedTargets() []actionTarget {
	targets := make([]actionTarget, 0, len(m.selected))
	for _, container := range m.snapshot.Containers {
		if _, selected := m.selected[container.ID]; selected {
			targets = append(targets, m.containerActionTarget(container))
		}
	}

	return targets
}

func (m Model) containerActionTarget(container domain.Container) actionTarget {
	target := actionTarget{ID: container.ID, Name: container.Name, State: container.State, ImageID: container.ImageID, Update: container.Update}
	if container.ComposeProject != "" {
		for _, stack := range m.stacks {
			if stack.Name == container.ComposeProject {
				stackCopy := stack
				target.Stack = &stackCopy
				break
			}
		}
		target.Service = container.ComposeService
		if container.ComposeOneOff {
			target.Unavailable = "Compose one-off containers cannot update the managed service"
		}
		target.UpdatePending = container.UpdatePending
		target.UpdateUnknown = container.UpdateUnknown
		if container.Update == domain.UpdateAvailable || container.Update == domain.UpdatePulledPendingRecreate || target.UpdatePending && target.UpdateUnknown {
			target.PullRefs = []string{container.Image}
		}
		return target
	}
	if container.Update == domain.UpdateAvailable {
		target.PullRefs = []string{container.Image}
		target.Recreate = []application.RecreateTarget{{ID: container.ID, Reference: container.Image}}
	}
	if pending, found := m.pendingRecreates[container.ID]; found && !pending.Compose {
		target.Update = domain.UpdatePulledPendingRecreate
		target.Recreate = []application.RecreateTarget{{ID: container.ID, Reference: pending.Reference}}
	}
	return target
}

func (m Model) runContainerUpdate(ctx context.Context, action application.Action, targets []actionTarget) []application.ActionResult {
	direct := m.containerUpdateTargets(action, targets)
	results := m.runDirectContainerUpdate(ctx, action, direct)
	return append(results, m.runComposeContainerUpdate(ctx, action, targets)...)
}

func (m Model) runDirectContainerUpdate(ctx context.Context, action application.Action, targets []application.RecreateTarget) []application.ActionResult {
	switch action {
	case application.ActionPull:
		return m.service.PullContainerUpdates(ctx, targets)
	case application.ActionUpdate:
		return m.service.UpdateContainers(ctx, targets)
	default:
		return m.service.ApplyContainerUpdates(ctx, targets)
	}
}

func (m Model) runComposeContainerUpdate(ctx context.Context, action application.Action, targets []actionTarget) []application.ActionResult {
	byStack := make(map[string]*application.StackUpdateTarget)
	services := make(map[string]map[string]struct{})
	for _, target := range targets {
		if target.Stack == nil || !targetUpdateEligible(action, target) {
			continue
		}
		update := byStack[target.Stack.Name]
		if update == nil {
			update = &application.StackUpdateTarget{Stack: *target.Stack, References: make(map[string]string)}
			byStack[target.Stack.Name] = update
			services[target.Stack.Name] = make(map[string]struct{})
		}
		if target.Service != "" {
			services[target.Stack.Name][target.Service] = struct{}{}
			if len(target.PullRefs) > 0 {
				update.References[target.Service] = target.PullRefs[0]
			}
		}
	}
	updates := make([]application.StackUpdateTarget, 0, len(byStack))
	for name, update := range byStack {
		for service := range services[name] {
			update.Services = append(update.Services, service)
		}
		sort.Strings(update.Services)
		updates = append(updates, *update)
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].Stack.Name < updates[j].Stack.Name })
	return m.service.UpdateStackServices(ctx, action, updates)
}

func (m Model) containerUpdateTargets(action application.Action, targets []actionTarget) []application.RecreateTarget {
	result := make([]application.RecreateTarget, 0, len(targets))
	for _, target := range targets {
		if target.Stack != nil || !targetUpdateEligible(action, target) {
			continue
		}
		result = append(result, target.Recreate...)
	}
	return result
}

func (m Model) containerTargetsForIDs(ids []string) []actionTarget {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	targets := make([]actionTarget, 0, len(ids))
	for _, container := range m.snapshot.Containers {
		if _, found := selected[container.ID]; found {
			targets = append(targets, m.containerActionTarget(container))
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
	choices := []application.Action{}
	if anyUpdateTargetEligible(targets, application.ActionPull) {
		choices = append(choices, application.ActionPull)
	}
	if anyUpdateTargetEligible(targets, application.ActionUpdate) {
		choices = append(choices, application.ActionUpdate)
	}
	if anyUpdateTargetEligible(targets, application.ActionApply) {
		choices = append(choices, application.ActionApply)
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
		return append(choices, "cancel")
	}
	state := strings.ToLower(targets[0].State)
	for _, target := range targets[1:] {
		if strings.ToLower(target.State) != state {
			return append(choices, "cancel")
		}
	}
	switch state {
	case "down", "missing compose file":
		if !anyPendingComposeTarget(targets) {
			choices = append(choices, application.ActionUp)
		}
	case "running", "mixed":
		choices = append(choices, application.ActionStop, application.ActionRestart, application.ActionDown)
	case "stopped":
		if !anyPendingComposeTarget(targets) {
			choices = append(choices, application.ActionUp)
		}
		choices = append(choices, application.ActionRestart, application.ActionDown)
	default:
		return []application.Action{"cancel"}
	}
	return append(choices, "cancel")
}

func anyUpdateTargetEligible(targets []actionTarget, action application.Action) bool {
	for _, target := range targets {
		if targetUpdateEligible(action, target) {
			return true
		}
	}
	return false
}

func anyPendingComposeTarget(targets []actionTarget) bool {
	for _, target := range targets {
		if target.UpdatePending {
			return true
		}
	}
	return false
}

func (m Model) actionChoices() []application.Action {
	if m.action.resource == actionImages {
		for _, target := range m.action.targets {
			if len(target.Recreate) > 0 {
				return []application.Action{application.ActionRecreate, application.ActionDelete, "cancel"}
			}
		}
		for _, target := range m.action.targets {
			if len(target.PullRefs) > 0 {
				return []application.Action{application.ActionPull, application.ActionDelete, "cancel"}
			}
		}
	}
	if m.action.resource == actionContainers || m.action.resource == actionStackContainers {
		choices := make([]application.Action, 0, 7)
		if m.anyEligibleTarget(application.ActionPull) {
			choices = append(choices, application.ActionPull)
		}
		if m.anyEligibleTarget(application.ActionUpdate) {
			choices = append(choices, application.ActionUpdate)
		}
		if m.anyEligibleTarget(application.ActionApply) {
			choices = append(choices, application.ActionApply)
		}
		choices = append(choices, actionChoices(m.action.resource)...)
		return choices
	}
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
	return actionResourceLabelLocalized(resource, i18n.New("en"))
}

func actionResourceLabelLocalized(resource actionResource, localizer sharedui.Localizer) string {
	switch resource {
	case actionImages:
		return localizer.Text(i18n.MessageTabImages)
	case actionNetworks:
		return localizer.Text(i18n.MessageTabNetworks)
	case actionVolumes:
		return localizer.Text(i18n.MessageTabVolumes)
	case actionStacks:
		return localizer.Text(i18n.MessageTabStacks)
	case actionStackContainers:
		return localizer.Text(i18n.MessageResourceStackContainersLabel)
	}
	return localizer.Text(i18n.MessageTabContainers)
}

func actionLabel(action application.Action) string {
	return actionLabelLocalized(action, i18n.New("en"))
}

func actionLabelLocalized(action application.Action, localizer sharedui.Localizer) string {
	switch action {
	case application.ActionStop:
		return localizer.Text(i18n.MessageActionStop)
	case application.ActionRestart:
		return localizer.Text(i18n.MessageActionRestart)
	case application.ActionDelete:
		return localizer.Text(i18n.MessageActionForceDelete)
	case application.ActionDown:
		return localizer.Text(i18n.MessageActionDownStack)
	case application.ActionUp:
		return localizer.Text(i18n.MessageActionUpStack)
	case application.ActionPull:
		return localizer.Text(i18n.MessageActionPullUpdate)
	case application.ActionRecreate:
		return localizer.Text(i18n.MessageActionRecreate)
	case application.ActionUpdate:
		return localizer.Text(i18n.MessageActionUpdateNow)
	case application.ActionApply:
		return localizer.Text(i18n.MessageActionApplyUpdate)
	default:
		return localizer.Text(i18n.MessageActionCancel)
	}
}

func actionLabelForResource(action application.Action, resource actionResource) string {
	return actionLabelForResourceLocalized(action, resource, i18n.New("en"))
}

func actionLabelForResourceLocalized(action application.Action, resource actionResource, localizer sharedui.Localizer) string {
	if (resource == actionImages || resource == actionNetworks || resource == actionVolumes) && action == application.ActionDelete {
		return localizer.Text(i18n.MessageActionDelete)
	}
	return actionLabelLocalized(action, localizer)
}

func (m Model) actionView() sharedui.View {
	if m.action.stage == actionMenu {
		return m.actionMenuView()
	}
	return sharedui.View{}
}

func (m Model) targetNames(targets []actionTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		line := "- " + target.Name
		if target.Unavailable != "" {
			line += m.localizer.Text(i18n.MessageActionTargetUnavailable, target.Unavailable)
		} else if m.action.resource == actionStacks && target.State != "" {
			line += m.localizer.Text(i18n.MessageActionTargetUnavailable, target.State)
		}
		names = append(names, line)
	}

	return names
}

func (m Model) confirmationBanner() string {
	if m.advanced.stage == advancedConfirm {
		kind, ok := m.selectedPruneKind()
		if !ok {
			return ""
		}
		return strings.Join([]string{
			m.localizer.Text(i18n.MessageAdvancedConfirmTitle),
			m.localizer.Text(i18n.MessageAdvancedCommand, application.PruneCommandText(kind)),
			m.localizer.Text(i18n.MessageAdvancedConfirmInput, m.advanced.input),
			m.localizer.Text(i18n.MessageAdvancedConfirmControls),
		}, "\n")
	}
	if m.action.stage != actionConfirm {
		return ""
	}
	action := actionLabelForResourceLocalized(m.selectedAction(), m.action.resource, m.localizer)
	targets := m.confirmationTargets(m.selectedAction())
	names := make([]string, 0, len(targets))
	for index, name := range targets {
		if index == 3 {
			names = append(names, m.localizer.Text(i18n.MessageConfirmMore, len(targets)-index))
			break
		}
		names = append(names, name)
	}
	lines := []string{
		m.localizer.Text(i18n.MessageConfirmTitle, action),
		m.localizer.Text(i18n.MessageConfirmTarget, strings.Join(names, ", "), engineTargetLocalized(m.snapshot.Engine, m.localizer)),
	}
	if isContainerUpdateAction(m.selectedAction()) {
		eligible := 0
		for _, target := range m.action.targets {
			if targetUpdateEligible(m.selectedAction(), target) {
				eligible++
			}
		}
		if eligible < len(m.action.targets) {
			lines = append(lines, m.localizer.Text(i18n.MessageActionEligibility, eligible, len(m.action.targets)-eligible))
		}
	}
	return strings.Join(append(lines, confirmationControlsLocalized(m.selectedAction(), m.localizer)), "\n")
}

func (m Model) confirmationTargets(action application.Action) []string {
	if !isContainerUpdateAction(action) {
		result := make([]string, 0, len(m.action.targets))
		for _, target := range m.action.targets {
			result = append(result, target.Name)
		}
		return result
	}
	seen := make(map[string]struct{}, len(m.action.targets))
	result := make([]string, 0, len(m.action.targets))
	for _, target := range m.action.targets {
		name := target.Name
		if target.Stack != nil {
			name = target.Stack.Name
			if target.Service != "" {
				name += "/" + target.Service
			}
		}
		if _, found := seen[name]; found {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func confirmationControls(action application.Action) string {
	return confirmationControlsLocalized(action, i18n.New("en"))
}

func confirmationControlsLocalized(_ application.Action, localizer sharedui.Localizer) string {
	return localizer.Text(i18n.MessageConfirmControls)
}

func (m *Model) showActionResult(results []application.ActionResult) {
	succeeded := 0
	for _, result := range results {
		if result.Err == nil {
			succeeded++
		}
	}
	if succeeded == len(results) {
		m.notice = m.localizer.Plural(i18n.MessageResultCompleted, succeeded)
	} else {
		m.notice = m.localizer.Text(i18n.MessageResultPartial, succeeded, len(results)-succeeded)
	}
	warnings := make([]string, 0)
	for _, result := range results {
		if result.Warning != nil {
			warnings = append(warnings, result.Warning.Error())
		}
	}
	if len(warnings) > 0 {
		m.notice = m.localizer.Text(i18n.MessageResultWarning, m.notice, strings.Join(warnings, "; "))
	}
	m.noticeGeneration++
}

func (m *Model) markPulledImages(targets []actionTarget, results []application.ActionResult) {
	if m.pendingRecreates == nil {
		m.pendingRecreates = make(map[string]pendingImageRecreate)
	}
	failed := make(map[string]struct{}, len(results))
	for _, result := range results {
		if result.Err != nil {
			failed[result.ID] = struct{}{}
		}
	}
	for _, target := range targets {
		for _, container := range m.snapshot.Containers {
			if container.State != "running" || normalizedImageID(container.ImageID) != normalizedImageID(target.ID) {
				continue
			}
			for _, reference := range target.PullRefs {
				if reference != container.Image {
					continue
				}
				if _, found := failed[reference]; !found {
					m.pendingRecreates[container.ID] = pendingImageRecreate{ContainerID: container.ID, ImageID: container.ImageID, Reference: reference, Compose: container.ComposeProject != ""}
					m.updates.Invalidate(reference)
				}
			}
		}
	}
	m.applyPendingImageStatuses()
}

func (m *Model) clearRecreatedImages(targets []actionTarget, results []application.ActionResult) {
	failed := make(map[string]struct{}, len(results))
	for _, result := range results {
		if result.Err != nil {
			failed[result.ID] = struct{}{}
		}
	}
	for _, target := range targets {
		for _, container := range target.Recreate {
			if _, found := failed[container.ID]; !found {
				delete(m.pendingRecreates, container.ID)
				m.updates.Invalidate(container.Reference)
			}
		}
	}
	m.applyPendingImageStatuses()
}

func normalizedImageID(id string) string {
	return strings.TrimPrefix(strings.ToLower(id), "sha256:")
}

func (m Model) containerPendingRecreate(containerID string) bool {
	_, found := m.pendingRecreates[containerID]
	return found
}

func (m Model) imagePendingRecreate(imageID string) bool {
	imageID = normalizedImageID(imageID)
	for _, pending := range m.pendingRecreates {
		if normalizedImageID(pending.ImageID) == imageID {
			return true
		}
	}
	return false
}

func (m *Model) applyPendingImageStatuses() {
	for index := range m.images {
		if m.imagePendingRecreate(m.images[index].ID) {
			m.images[index].Update = domain.UpdatePulledPendingRecreate
		}
	}
}

func (m *Model) applyContainerUpdateStatuses() {
	for index := range m.snapshot.Containers {
		container := &m.snapshot.Containers[index]
		container.Update = ""
		if container.State == "running" {
			container.Update = m.containerUpdates[container.ID]
		}
		if _, found := m.pendingRecreates[container.ID]; found {
			container.Update = domain.UpdatePulledPendingRecreate
		}
	}
}

func aggregateUpdateStatus(current, next domain.UpdateStatus) domain.UpdateStatus {
	rank := func(status domain.UpdateStatus) int {
		switch status {
		case domain.UpdatePulledPendingRecreate:
			return 6
		case domain.UpdateAvailable:
			return 5
		case domain.UpdateChecking:
			return 4
		case domain.UpdateUnknown:
			return 3
		case domain.UpdatePinned:
			return 2
		case domain.UpdateCurrent:
			return 1
		default:
			return 0
		}
	}
	if rank(next) > rank(current) {
		return next
	}
	return current
}

func (m *Model) prunePendingRecreates() {
	available := make(map[string]struct{}, len(m.snapshot.Containers))
	for _, container := range m.snapshot.Containers {
		available[container.ID] = struct{}{}
	}
	for id := range m.pendingRecreates {
		if _, found := available[id]; !found {
			delete(m.pendingRecreates, id)
		}
	}
}

func stackUpdateStatus(stack domain.Stack) domain.UpdateStatus {
	if stack.UpdatePending {
		return stack.Update
	}
	status := stack.Update
	for _, container := range stack.ContainerItems {
		if container.Update == domain.UpdatePulledPendingRecreate {
			return domain.UpdatePulledPendingRecreate
		}
		if container.Update == domain.UpdateAvailable {
			status = domain.UpdateAvailable
		}
	}
	return status
}

func (m *Model) reconcileContainerUpdateResults(action application.Action, targets []actionTarget, results []application.ActionResult) {
	if m.pendingRecreates == nil {
		m.pendingRecreates = make(map[string]pendingImageRecreate)
	}
	for _, result := range results {
		for _, target := range targets {
			if result.ID != target.ID && (target.Stack == nil || result.ID != target.Stack.Name) {
				continue
			}
			if target.Stack != nil {
				continue
			}
			containers := []actionTarget{target}
			if target.Stack != nil {
				containers = make([]actionTarget, 0, len(target.Stack.ContainerItems))
				for _, container := range target.Stack.ContainerItems {
					if target.Service != "" && container.ComposeService != target.Service {
						continue
					}
					if !result.Applied && action != application.ActionApply && container.Update != domain.UpdateAvailable && container.Update != domain.UpdatePulledPendingRecreate || !result.Applied && action == application.ActionApply && container.Update != domain.UpdatePulledPendingRecreate {
						continue
					}
					containers = append(containers, actionTarget{ID: container.ID, ImageID: container.ImageID, Update: container.Update, PullRefs: []string{container.Image}, Stack: target.Stack, Service: container.ComposeService})
				}
			}
			for _, container := range containers {
				reference := ""
				if len(container.PullRefs) > 0 {
					reference = container.PullRefs[0]
				}
				if len(container.Recreate) > 0 {
					reference = container.Recreate[0].Reference
				}
				if result.Pulled && !result.Applied {
					m.pendingRecreates[container.ID] = pendingImageRecreate{ContainerID: container.ID, ImageID: container.ImageID, Reference: reference, Compose: target.Stack != nil}
					m.containerUpdates[container.ID] = domain.UpdatePulledPendingRecreate
					m.setImageUpdate(container.ImageID, domain.UpdatePulledPendingRecreate)
				}
				if result.Applied {
					delete(m.pendingRecreates, container.ID)
					delete(m.containerUpdates, container.ID)
					m.setImageUpdate(container.ImageID, domain.UpdateUnknown)
				}
				if m.updates != nil && reference != "" {
					m.updates.Invalidate(reference)
				}
			}
		}
	}
	m.applyPendingImageStatuses()
	m.applyContainerUpdateStatuses()
}

func (m *Model) recordPendingRecreate(update domain.ImageUpdate) {
	if m.pendingRecreates == nil {
		m.pendingRecreates = make(map[string]pendingImageRecreate)
	}
	imageID := normalizedImageID(update.ImageID)
	for _, container := range m.snapshot.Containers {
		if container.ComposeProject != "" {
			continue
		}
		if update.ContainerID != "" && container.ID != update.ContainerID {
			continue
		}
		if container.State != "running" || normalizedImageID(container.ImageID) != imageID {
			continue
		}
		if _, ok := application.NormalizeImageReference(container.Image); !ok {
			continue
		}
		reference := container.Image
		if update.Reference != "" {
			reference = update.Reference
		}
		m.pendingRecreates[container.ID] = pendingImageRecreate{ContainerID: container.ID, ImageID: container.ImageID, Reference: reference}
	}
}

func (m *Model) setImageUpdate(imageID string, status domain.UpdateStatus) {
	imageID = normalizedImageID(imageID)
	for index := range m.images {
		if normalizedImageID(m.images[index].ID) == imageID {
			m.images[index].Update = status
		}
	}
}

func preserveImageUpdates(previous, images []domain.Image) []domain.Image {
	updates := make(map[string]domain.UpdateStatus, len(previous))
	for _, image := range previous {
		updates[image.ID] = image.Update
	}
	for index := range images {
		images[index].Update = updates[images[index].ID]
	}
	return images
}

func (m Model) expireActionNotice() tea.Cmd {
	generation := m.noticeGeneration
	return tea.Tick(actionResultDuration, func(time.Time) tea.Msg {
		return actionNoticeExpiredMsg{generation: generation}
	})
}

func shortContainerID(id string) string {
	if len(id) <= 12 {
		return id
	}

	return id[:12]
}

func engineTarget(engine domain.EngineInfo) string {
	return engineTargetLocalized(engine, i18n.New("en"))
}

func engineTargetLocalized(engine domain.EngineInfo, localizer sharedui.Localizer) string {
	scope := localizer.Text(i18n.MessageScopeLocal)
	if engine.Remote {
		scope = localizer.Text(i18n.MessageScopeRemote)
	}

	return scope + " " + engine.Name
}

func (m Model) actionResourceCount(count int) string {
	id := i18n.MessageResourceContainers
	switch m.action.resource {
	case actionImages:
		id = i18n.MessageResourceImages
	case actionNetworks:
		id = i18n.MessageResourceNetworks
	case actionVolumes:
		id = i18n.MessageResourceVolumes
	case actionStacks:
		id = i18n.MessageResourceStacks
	case actionStackContainers:
		id = i18n.MessageResourceStackContainers
	}
	return m.localizer.Plural(id, count)
}

func (m Model) containersView(layout sharedui.Layout) sharedui.View {
	title := m.localizer.Text(i18n.MessageTabContainers)
	if m.action.stage == actionMenu {
		return m.actionMenuView()
	}
	if m.panel == panelDetails {
		return m.detailsView()
	}
	if m.panel == panelLogs {
		return m.logsView(layout)
	}
	if m.shellActive {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageContainerShellTitle), Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageContainerShellStarting)}
	}
	if m.shellErr != nil {
		return sharedui.View{
			Title: title, Status: sharedui.StatusError, Summary: m.localizer.Text(i18n.MessageContainerShellFailed) + m.shellErr.Error(),
			Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageContainerShellFailureNext)}},
		}
	}
	if m.showHelp {
		sections := []sharedui.Section{
			{Title: m.localizer.Text(i18n.MessageSectionHelp), Body: m.localizer.Text(i18n.MessageContainersHelp)},
			{Title: m.localizer.Text(i18n.MessageSectionConnection), Body: engineDetailsLocalized(m.snapshot.Engine, m.localizer)},
		}
		if m.dockerHubLoginChecked && !m.dockerHubLoginConfigured {
			sections = append(sections, sharedui.Section{Title: m.localizer.Text(i18n.MessageSectionImageUpdates), Body: m.localizer.Text(i18n.MessageDockerHubHelp)})
		}
		return sharedui.View{
			Title:      title,
			Status:     sharedui.StatusReady,
			HideStatus: true,
			Sections:   sections,
		}
	}

	if m.loading {
		return sharedui.View{
			Title:   title,
			Status:  sharedui.StatusLoading,
			Summary: m.localizer.Text(i18n.MessageContainersLoading),
		}
	}

	if m.err != nil {
		status := sharedui.StatusError
		if errors.Is(m.err, domain.ErrRemoteUnsupported) {
			status = sharedui.StatusUnavailable
		}

		return sharedui.View{
			Title:   title,
			Status:  status,
			Summary: dockerErrorSummaryLocalized(m.err, m.localizer),
			Sections: []sharedui.Section{
				{Title: m.localizer.Text(i18n.MessageSectionConnection), Body: m.localizer.Text(i18n.MessageContainersConnectionFailure)},
				{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageContainersConnectionNext)},
			},
		}
	}

	if len(m.snapshot.Containers) == 0 {
		return sharedui.View{
			Title:   title,
			Status:  sharedui.StatusEmpty,
			Summary: m.localizer.Text(i18n.MessageContainersEmptySummary),
			Sections: []sharedui.Section{
				{Title: title, Body: m.localizer.Text(i18n.MessageContainersEmptyBody)},
			},
		}
	}

	return sharedui.View{
		Title:      title,
		Status:     sharedui.StatusReady,
		HideStatus: true,
		Sections: []sharedui.Section{
			{Body: renderContainersLocalized(m.snapshot.Containers, m.selectedID, m.selected, m.editing, m.now(), layout, m.memoryMode, m.accentColor, m.focusColor, m.localizer)},
		},
	}
}

func (m Model) imagesActionView() sharedui.View {
	if m.action.stage == actionMenu {
		return m.actionMenuView()
	}
	return sharedui.View{}
}

func (m Model) detailsView() sharedui.View {
	title := m.localizer.Text(i18n.MessageContainerDetails)
	if m.detailLoading {
		return sharedui.View{Title: title, Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageContainerDetailsLoad)}
	}
	if m.detailErr != nil {
		return sharedui.View{
			Title: title, Status: sharedui.StatusError, Summary: m.detailErr.Error(),
			Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageCommonBack)}},
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
		Title: title, Status: sharedui.StatusReady, HideStatus: true,
		Sections: []sharedui.Section{
			{Title: details.Name, Body: strings.Join([]string{m.localizer.Text(i18n.MessageDetailsID, shortContainerID(details.ID)), m.localizer.Text(i18n.MessageDetailsImage, details.Image), m.localizer.Text(i18n.MessageDetailsState, localizeState(m.localizer, details.State)), m.localizer.Text(i18n.MessageDetailsHealth, localizeHealth(m.localizer, details.Health)), m.localizer.Text(i18n.MessageDetailsUptime, formatUptime(details.StartedAt, details.State, m.now()))}, "\n")},
			{Title: m.localizer.Text(i18n.MessageSectionPorts), Body: ports},
			{Title: m.localizer.Text(i18n.MessageSectionNetworks), Body: networks},
			{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageFooterContainerDetails)},
		},
	}
}

func (m Model) logsView(layout sharedui.Layout) sharedui.View {
	status := m.localizer.Text(i18n.MessageLogsLoading)
	if m.logActive {
		status = m.localizer.Text(i18n.MessageLogsFollowing)
	}
	if m.logErr != nil {
		status = m.logErr.Error()
	}
	lines := visibleLogs(m.logLines, m.logOffset, logLineCount(layout))
	body := m.localizer.Text(i18n.MessageLogsEmpty)
	if len(lines) > 0 {
		body = strings.Join(lines, "\n")
	}
	return sharedui.View{
		Title: logPanelTitleLocalized(m.stackLogs, m.localizer), Status: sharedui.StatusReady, HideStatus: true, Summary: status,
		Sections: []sharedui.Section{
			{Title: m.logName(), Body: body},
			{Title: m.localizer.Text(i18n.MessageSectionControls), Body: m.localizer.Text(i18n.MessageLogsControls)},
		},
	}
}

func logPanelTitle(stack bool) string {
	return logPanelTitleLocalized(stack, i18n.New("en"))
}

func logPanelTitleLocalized(stack bool, localizer sharedui.Localizer) string {
	if stack {
		return localizer.Text(i18n.MessageLogsComposeTitle)
	}
	return localizer.Text(i18n.MessageLogsContainerTitle)
}
func (m Model) logName() string {
	if m.stackLogs {
		return m.logTitle
	}
	return m.selectedContainerName()
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
	return engineDetailsLocalized(info, i18n.New("en"))
}

func engineDetailsLocalized(info domain.EngineInfo, localizer sharedui.Localizer) string {
	return strings.Join([]string{
		localizer.Text(i18n.MessageEngineName, info.Name),
		localizer.Text(i18n.MessageEngineEndpoint, info.Endpoint),
		localizer.Text(i18n.MessageEngineTransport, info.Transport),
		localizer.Text(i18n.MessageEngineRemote, yesNoLocalized(info.Remote, localizer)),
		localizer.Text(i18n.MessageEngineSecure, yesNoLocalized(info.Secure, localizer)),
		localizer.Text(i18n.MessageEngineSource, info.Source),
		localizer.Text(i18n.MessageEngineServer, info.ServerVersion),
		localizer.Text(i18n.MessageEngineAPI, info.APIVersion),
		localizer.Text(i18n.MessageEngineOS, info.OperatingSystem),
		localizer.Text(i18n.MessageEngineCPUs, info.NCPU),
		localizer.Text(i18n.MessageEngineRAM, formatBytes(info.MemoryTotal)),
	}, "\n")
}

func dockerErrorSummary(err error) string {
	return dockerErrorSummaryLocalized(err, i18n.New("en"))
}

func dockerErrorSummaryLocalized(err error, localizer sharedui.Localizer) string {
	if errors.Is(err, domain.ErrRemoteUnsupported) {
		return localizer.Text(i18n.MessageDockerRemoteUnsupported)
	}

	return err.Error()
}

func yesNo(value bool) string {
	return yesNoLocalized(value, i18n.New("en"))
}

func yesNoLocalized(value bool, localizer sharedui.Localizer) string {
	if value {
		return localizer.Text(i18n.MessageCommonYes)
	}
	return localizer.Text(i18n.MessageCommonNo)
}

func (m Model) headerSummary() string {
	if m.loading {
		return m.localizer.Text(i18n.MessageHeaderConnecting)
	}
	if m.err != nil {
		return m.localizer.Text(i18n.MessageHeaderUnavailable)
	}

	running := 0
	for _, container := range m.snapshot.Containers {
		if container.State == "running" {
			running++
		}
	}

	scope := m.localizer.Text(i18n.MessageScopeLocal)
	if m.snapshot.Engine.Remote {
		scope = m.localizer.Text(i18n.MessageScopeRemote)
	}
	cpu := "--"
	if m.snapshot.CPUAvailable {
		cpu = m.localizer.Decimal(m.snapshot.ContainerCPUPercent, 1) + "%"
	}
	memory := "--/" + formatBytes(m.snapshot.Engine.MemoryTotal)
	if m.snapshot.MemoryAvailable {
		memory = formatBytes(m.snapshot.ContainerMemoryUsage) + "/" + formatBytes(m.snapshot.Engine.MemoryTotal)
	}

	return m.localizer.Text(i18n.MessageHeaderSummary, scope, m.snapshot.Engine.Name, cpu, memory, running, len(m.snapshot.Containers), m.localizer.Text(i18n.MessageHeaderRunning), sortLabelLocalized(m.sortMode, m.localizer), m.snapshot.Engine.ServerVersion)
}

func (m Model) footer(layout sharedui.Layout) string {
	if m.advanced.stage != advancedClosed {
		switch m.advanced.stage {
		case advancedConfirm:
			return m.localizer.Text(i18n.MessageAdvancedConfirmControls)
		case advancedRunning:
			return m.localizer.Text(i18n.MessageAdvancedRunning)
		case advancedResult:
			return m.localizer.Text(i18n.MessageAdvancedResultControls)
		default:
			return m.localizer.Text(i18n.MessageAdvancedControls)
		}
	}
	if m.action.stage == actionConfirm {
		return m.localizer.Text(i18n.MessageFooterConfirmation)
	}
	if m.notice != "" {
		return m.notice
	}
	if m.active == 1 {
		if m.selectedStackContainerID != "" && m.expandedStackName == m.selectedStackName {
			if m.stackContainerEditing {
				return m.localizer.Text(i18n.MessageFooterEdit, len(m.selectedStackContainers))
			}
			return m.localizer.Text(i18n.MessageFooterStackChild)
		}
		if m.stackEditing {
			return m.localizer.Text(i18n.MessageFooterEdit, len(m.selectedStacks))
		}
		return m.localizer.Text(i18n.MessageFooterStack)
	}
	if m.active == 2 {
		if m.action.resource == actionImages && m.action.stage != actionNone {
			return m.localizer.Text(i18n.MessageFooterActions)
		}
		if m.imageDetailOpen {
			return m.localizer.Text(i18n.MessageFooterDetails)
		}
		if m.imageEditing {
			return m.localizer.Text(i18n.MessageFooterEdit, len(m.selectedImages))
		}
		return m.localizer.Text(i18n.MessageFooterResource)
	}
	if m.active == 3 {
		if m.action.resource == actionNetworks && m.action.stage != actionNone {
			return m.localizer.Text(i18n.MessageFooterActions)
		}
		if m.networkDetailOpen {
			return m.localizer.Text(i18n.MessageFooterDetails)
		}
		if m.networkEditing {
			return m.localizer.Text(i18n.MessageFooterEdit, len(m.selectedNetworks))
		}
		return m.localizer.Text(i18n.MessageFooterResource)
	}
	if m.active == 4 {
		if m.action.resource == actionVolumes && m.action.stage != actionNone {
			return m.localizer.Text(i18n.MessageFooterActions)
		}
		if m.volumeDetailOpen {
			return m.localizer.Text(i18n.MessageFooterDetails)
		}
		if m.volumeEditing {
			return m.localizer.Text(i18n.MessageFooterEdit, len(m.selectedVolumes))
		}
		return m.localizer.Text(i18n.MessageFooterResource)
	}
	if m.panel == panelLogs {
		return m.localizer.Text(i18n.MessageFooterLogs)
	}
	if m.panel == panelDetails {
		return m.localizer.Text(i18n.MessageFooterContainerDetails)
	}
	if m.editing {
		return m.localizer.Text(i18n.MessageFooterEdit, len(m.selected))
	}
	switch layout.Mode {
	case sharedui.LayoutMinimal:
		return m.localizer.Text(i18n.MessageFooterMinimal)
	case sharedui.LayoutCompact:
		return m.localizer.Text(i18n.MessageFooterCompact)
	default:
		return m.localizer.Text(i18n.MessageFooterDefault)
	}
}

func (m Model) dockerHubFooterNotice() string {
	if m.dockerHubLoginChecked && !m.dockerHubLoginConfigured {
		return m.localizer.Text(i18n.MessageDockerHubFooter)
	}
	return ""
}

func sortLabel(mode application.SortMode) string {
	return sortLabelLocalized(mode, i18n.New("en"))
}

func sortLabelLocalized(mode application.SortMode, localizer sharedui.Localizer) string {
	switch mode {
	case application.SortCPU:
		return localizer.Text(i18n.MessageSortCPU)
	case application.SortMemory:
		return localizer.Text(i18n.MessageSortMemory)
	case application.SortName:
		return localizer.Text(i18n.MessageSortName)
	default:
		return localizer.Text(i18n.MessageSortState)
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
