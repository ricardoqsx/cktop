package i18n

import (
	"fmt"
	"strconv"
	"strings"

	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

type message struct {
	text  string
	one   string
	other string
}

var english = map[string]message{
	sharedui.MessageShellFallbackTitle:       {text: "cktop"},
	sharedui.MessageShellFallbackViewTitle:   {text: "Overview"},
	sharedui.MessageShellFallbackViewSummary: {text: "No content available yet."},
	sharedui.MessageShellStatusReady:         {text: "ready"},
	sharedui.MessageShellStatusLoading:       {text: "loading"},
	sharedui.MessageShellStatusEmpty:         {text: "empty"},
	sharedui.MessageShellStatusWarning:       {text: "warning"},
	sharedui.MessageShellStatusError:         {text: "error"},
	sharedui.MessageShellStatusUnavailable:   {text: "unavailable"},
	sharedui.MessageShellKeyQuit:             {text: "quit"},
	sharedui.MessageShellKeyNext:             {text: "next"},
	sharedui.MessageShellKeyPrevious:         {text: "prev"},
	sharedui.MessageShellKeyHelp:             {text: "help"},
	sharedui.MessageShellKeyBack:             {text: "back"},
	sharedui.MessageShellHelpTitle:           {text: "Help"},
	sharedui.MessageShellHelpNextView:        {text: "Tab        next view"},
	sharedui.MessageShellHelpPreviousView:    {text: "Shift+Tab  previous view"},
	sharedui.MessageShellHelpClose:           {text: "Esc        close help"},
	sharedui.MessageShellHelpQuit:            {text: "q          quit"},
	sharedui.MessageShellHelpCompact:         {text: "Help: tab next | shift+tab prev | esc close | q quit"},
	sharedui.MessageShellFooterDefault:       {text: "tab next  shift+tab prev  ? help  q quit"},
	sharedui.MessageShellFooterMinimal:       {text: "tab next  q quit"},
	sharedui.MessageShellTabPosition:         {text: " %d/%d"},
	sharedui.MessageShellViewCount:           {one: "%d view", other: "%d views"},
	MessageTabContainers:                     {text: "Containers"}, MessageTabStacks: {text: "Stacks"}, MessageTabImages: {text: "Images"}, MessageTabNetworks: {text: "Networks"}, MessageTabVolumes: {text: "Volumes"},
	MessageSectionNext: {text: "Next"}, MessageSectionControls: {text: "Controls"}, MessageSectionHelp: {text: "Help"}, MessageSectionConnection: {text: "Connection"}, MessageSectionImageUpdates: {text: "Image updates"}, MessageSectionPorts: {text: "Ports"}, MessageSectionNetworks: {text: "Networks"}, MessageSectionContainers: {text: "Containers"}, MessageSectionTags: {text: "Tags"}, MessageSectionDigests: {text: "Digests"}, MessageSectionOptions: {text: "Options"}, MessageSectionAction: {text: "Action"}, MessageSectionSelectedStack: {text: "Selected stack"}, MessageSectionRegistrationDiagnostics: {text: "Registration diagnostics"}, MessageSectionUpdateEligibility: {text: "Update selection"},
	MessageCommonRetry: {text: "Press r to retry."}, MessageCommonBack: {text: "esc back"}, MessageCommonYes: {text: "yes"}, MessageCommonNo: {text: "no"}, MessageCommonAvailable: {text: "available"}, MessageCommonUnavailableReason: {text: "unavailable: %s"}, MessageCommonUnknown: {text: "unknown"}, MessageCommonDangling: {text: "dangling"}, MessageCommonUntagged: {text: "<untagged>"},
	MessageContainersLoading: {text: "Connecting to Docker Engine and loading containers..."}, MessageContainersEmptySummary: {text: "Connected to Docker Engine, but no containers were found."}, MessageContainersEmptyBody: {text: "No containers. Try creating one with Docker, then press r to retry."}, MessageContainersConnectionFailure: {text: "dtop could not connect to a supported local Docker Engine."}, MessageContainersConnectionNext: {text: "Check Docker Desktop, docker context, DOCKER_HOST, socket permissions, or daemon status. Press r to retry."}, MessageContainersHelp: {text: "enter actions | d details | s shell | e edit | space select | a select all | up/down move | o sort | r refresh | left/right view | esc close | q quit"}, MessageDockerHubHelp: {text: "To get updates for your container images, log in to Docker Hub with: docker login"}, MessageDockerHubFooter: {text: "Docker Hub: run docker login to check image updates reliably"},
	MessageImagesLoading: {text: "Loading Docker images..."}, MessageImagesPartial: {text: "Showing last known images: "}, MessageImagesEmpty: {text: "No Docker images were found."}, MessageImageDetailsTitle: {text: "Image details"}, MessageImageDetailsLoad: {text: "Loading image details..."}, MessageImageDetailsSize: {text: "Size: %s"}, MessageImageDetailsCreate: {text: "Created: %s"}, MessageImageDetailsPlat: {text: "Platform: %s"},
	MessageNetworksLoading: {text: "Loading Docker networks..."}, MessageNetworksPartial: {text: "Showing last known networks: "}, MessageNetworksEmpty: {text: "No Docker networks were found."}, MessageNetworkDetailsTitle: {text: "Network details"}, MessageNetworkDetailsLoad: {text: "Loading network details..."},
	MessageVolumesLoading: {text: "Loading Docker volumes..."}, MessageVolumesPartial: {text: "Showing last known volumes: "}, MessageVolumesEmpty: {text: "No Docker volumes were found."}, MessageVolumeDetailsTitle: {text: "Volume details"}, MessageVolumeDetailsLoad: {text: "Loading volume details..."},
	MessageStacksLoading: {text: "Loading Docker Compose stacks..."}, MessageStacksEmpty: {text: "No stacks found. Only stacks with Docker Compose labels can be discovered."}, MessageStackWorkingDirectory: {text: "Working directory: %s"}, MessageStackComposeFiles: {text: "Compose files: %s"}, MessageStackDown: {text: "Down: %s"}, MessageStackContainerFallback: {text: "container"}, MessageContainerShellTitle: {text: "Container shell"}, MessageContainerShellStarting: {text: "Starting an interactive shell..."}, MessageContainerShellFailed: {text: "Container shell failed: "}, MessageContainerShellFailureNext: {text: "Check that the container is running, /bin/sh exists, and Docker permissions allow exec. Press s to try again."},
	MessageDetailsID: {text: "ID: %s"}, MessageDetailsImage: {text: "Image: %s"}, MessageDetailsState: {text: "State: %s"}, MessageDetailsHealth: {text: "Health: %s"}, MessageDetailsUptime: {text: "Uptime: %s"}, MessageDetailsDriver: {text: "Driver: %s"}, MessageDetailsScope: {text: "Scope: %s"}, MessageDetailsCreated: {text: "Created: %s"}, MessageDetailsInternal: {text: "Internal: %s"}, MessageDetailsAttachable: {text: "Attachable: %s"}, MessageDetailsMountpoint: {text: "Mountpoint: %s"}, MessageContainerDetails: {text: "Container details"}, MessageContainerDetailsLoad: {text: "Loading container details..."},
	MessageEngineName: {text: "Name: %s"}, MessageEngineEndpoint: {text: "Endpoint: %s"}, MessageEngineTransport: {text: "Transport: %s"}, MessageEngineRemote: {text: "Remote: %s"}, MessageEngineSecure: {text: "Secure: %s"}, MessageEngineSource: {text: "Source: %s"}, MessageEngineServer: {text: "Server: %s"}, MessageEngineAPI: {text: "API: %s"}, MessageEngineOS: {text: "OS: %s"}, MessageEngineCPUs: {text: "CPUs: %d"}, MessageEngineRAM: {text: "RAM: %s"},
	MessageLogsLoading: {text: "Loading log stream..."}, MessageLogsFollowing: {text: "Following live logs"}, MessageLogsEmpty: {text: "(no log output)"}, MessageLogsComposeTitle: {text: "Compose logs"}, MessageLogsContainerTitle: {text: "Container logs"}, MessageLogsControls: {text: "up/down scroll | esc stop and back"},
	MessageDockerRemoteUnsupported: {text: "A remote Docker endpoint is configured, but D1A only supports local Engines. Remote support is planned for D1R."}, MessageHeaderConnecting: {text: "connecting Docker Engine"}, MessageHeaderUnavailable: {text: "Docker unavailable"}, MessageHeaderSummary: {text: "%s %s | CPU %s | RAM %s | %d/%d %s | SORT: %s | Docker %s"}, MessageHeaderRunning: {text: "running"}, MessageScopeLocal: {text: "LOCAL"}, MessageScopeRemote: {text: "REMOTE"}, MessageSortCPU: {text: "CPU"}, MessageSortMemory: {text: "Memory"}, MessageSortName: {text: "Name"}, MessageSortState: {text: "State"},
	MessageColumnName: {text: "NAME"}, MessageColumnState: {text: "STATE"}, MessageColumnCPU: {text: "CPU"}, MessageColumnMemory: {text: "MEM"}, MessageColumnHealth: {text: "HEALTH"}, MessageColumnUptime: {text: "UPTIME"}, MessageColumnImage: {text: "IMAGE"}, MessageColumnID: {text: "ID"}, MessageColumnUpdate: {text: "UPDATE"}, MessageColumnSize: {text: "SIZE"}, MessageColumnUsed: {text: "USED"}, MessageColumnAge: {text: "AGE"}, MessageColumnDriver: {text: "DRIVER"}, MessageColumnScope: {text: "SCOPE"}, MessageColumnServices: {text: "SERVICES"}, MessageColumnContainers: {text: "CONTAINERS"},
	MessageUsageContainers: {one: "%d container", other: "%d containers"}, MessageResourceContainers: {one: "%d container", other: "%d containers"}, MessageResourceImages: {one: "%d image", other: "%d images"}, MessageResourceNetworks: {one: "%d network", other: "%d networks"}, MessageResourceVolumes: {one: "%d volume", other: "%d volumes"}, MessageResourceStacks: {one: "%d stack", other: "%d stacks"}, MessageResourceStackContainers: {one: "%d stack container", other: "%d stack containers"}, MessageResourceStackContainersLabel: {text: "Stack containers"},
	MessageActionSelected: {text: "Selected: %s"}, MessageActionControls: {text: "up/down choose | enter continue | esc cancel"}, MessageActionStop: {text: "Stop"}, MessageActionRestart: {text: "Restart"}, MessageActionForceDelete: {text: "Force delete"}, MessageActionDelete: {text: "Delete"}, MessageActionDownStack: {text: "Down stack"}, MessageActionUpStack: {text: "Up stack"}, MessageActionPullUpdate: {text: "Pull update"}, MessageActionRecreate: {text: "Recreate containers"}, MessageActionUpdateNow: {text: "Update now"}, MessageActionApplyUpdate: {text: "Apply downloaded update"}, MessageActionEligibility: {text: "%d eligible, %d skipped"}, MessageActionCancel: {text: "Cancel"}, MessageActionTargetUnavailable: {text: " (unavailable: %s)"}, MessageConfirmTitle: {text: "CONFIRM: %s"}, MessageConfirmTarget: {text: "Target: %s on %s"}, MessageConfirmMore: {text: "+%d more"}, MessageConfirmControls: {text: "Are you sure? [y/N] | esc cancel"}, MessageResultCompleted: {one: "%d action completed", other: "%d actions completed"}, MessageResultPartial: {text: "%d completed, %d failed"},
	MessageFooterConfirmation: {text: "Confirmation below"}, MessageFooterEdit: {text: "EDIT: %d selected | space toggle | a all/none | enter actions | e/esc cancel"}, MessageFooterStackChild: {text: "s shell  l logs  enter actions  esc collapse  up/down select  r refresh  left/right views  q quit"}, MessageFooterStack: {text: "enter expand  esc collapse  up/down select  r refresh  left/right views  q quit"}, MessageFooterActions: {text: "up/down choose  enter continue  esc cancel"}, MessageFooterDetails: {text: "esc back  left/right views  q quit"}, MessageFooterResource: {text: "enter actions  d details  e edit  up/down select  r refresh  left/right views  q quit"}, MessageFooterLogs: {text: "up/down scroll  esc stop and back  q quit"}, MessageFooterContainerDetails: {text: "l logs  esc back  q quit"}, MessageFooterMinimal: {text: "e edit  q quit  up/down"}, MessageFooterCompact: {text: "s shell  e edit  up/down select  o sort  r refresh  q quit"}, MessageFooterDefault: {text: "enter actions  d details  s shell  l logs  e edit  up/down select  o sort  r refresh  left/right views  ? help  q quit"},
	MessageKeyQuit: {text: "quit"}, MessageKeyNext: {text: "next view"}, MessageKeyPrevious: {text: "previous view"}, MessageKeyRetry: {text: "retry"}, MessageKeyUp: {text: "up"}, MessageKeyDown: {text: "down"}, MessageKeyHelp: {text: "help"}, MessageKeyBack: {text: "back"}, MessageKeySort: {text: "sort"}, MessageKeyEdit: {text: "edit"}, MessageKeyAll: {text: "all"}, MessageKeySelect: {text: "select"}, MessageKeyActions: {text: "actions"}, MessageKeyDetails: {text: "details"}, MessageKeyLogs: {text: "logs"}, MessageKeyShell: {text: "shell"},
	MessageStateRunning: {text: "running"}, MessageStateStopped: {text: "stopped"}, MessageStateExited: {text: "exited"}, MessageStateMixed: {text: "mixed"}, MessageStateDown: {text: "down"}, MessageStateMissingComposeFile: {text: "missing compose file"}, MessageHealthHealthy: {text: "healthy"}, MessageHealthUnhealthy: {text: "unhealthy"}, MessageHealthStarting: {text: "starting"},
}

var spanish = map[string]message{
	sharedui.MessageShellFallbackTitle:       {text: "cktop"},
	sharedui.MessageShellFallbackViewTitle:   {text: "Resumen"},
	sharedui.MessageShellFallbackViewSummary: {text: "Todavia no hay contenido disponible."},
	sharedui.MessageShellStatusReady:         {text: "listo"},
	sharedui.MessageShellStatusLoading:       {text: "cargando"},
	sharedui.MessageShellStatusEmpty:         {text: "vacio"},
	sharedui.MessageShellStatusWarning:       {text: "aviso"},
	sharedui.MessageShellStatusError:         {text: "error"},
	sharedui.MessageShellStatusUnavailable:   {text: "no disponible"},
	sharedui.MessageShellKeyQuit:             {text: "salir"},
	sharedui.MessageShellKeyNext:             {text: "siguiente"},
	sharedui.MessageShellKeyPrevious:         {text: "anterior"},
	sharedui.MessageShellKeyHelp:             {text: "ayuda"},
	sharedui.MessageShellKeyBack:             {text: "volver"},
	sharedui.MessageShellHelpTitle:           {text: "Ayuda"},
	sharedui.MessageShellHelpNextView:        {text: "Tab        vista siguiente"},
	sharedui.MessageShellHelpPreviousView:    {text: "Shift+Tab  vista anterior"},
	sharedui.MessageShellHelpClose:           {text: "Esc        cerrar ayuda"},
	sharedui.MessageShellHelpQuit:            {text: "q          salir"},
	sharedui.MessageShellHelpCompact:         {text: "Ayuda: tab siguiente | shift+tab anterior | esc cerrar | q salir"},
	sharedui.MessageShellFooterDefault:       {text: "tab siguiente  shift+tab anterior  ? ayuda  q salir"},
	sharedui.MessageShellFooterMinimal:       {text: "tab siguiente  q salir"},
	sharedui.MessageShellTabPosition:         {text: " %d/%d"},
	sharedui.MessageShellViewCount:           {one: "%d vista", other: "%d vistas"},
	MessageTabContainers:                     {text: "Contenedores"}, MessageTabStacks: {text: "Stacks"}, MessageTabImages: {text: "Imagenes"}, MessageTabNetworks: {text: "Redes"}, MessageTabVolumes: {text: "Volumenes"},
	MessageSectionNext: {text: "Siguiente"}, MessageSectionControls: {text: "Controles"}, MessageSectionHelp: {text: "Ayuda"}, MessageSectionConnection: {text: "Conexion"}, MessageSectionImageUpdates: {text: "Actualizaciones de imagenes"}, MessageSectionPorts: {text: "Puertos"}, MessageSectionNetworks: {text: "Redes"}, MessageSectionContainers: {text: "Contenedores"}, MessageSectionTags: {text: "Etiquetas"}, MessageSectionDigests: {text: "Digests"}, MessageSectionOptions: {text: "Opciones"}, MessageSectionAction: {text: "Accion"}, MessageSectionSelectedStack: {text: "Stack seleccionado"}, MessageSectionRegistrationDiagnostics: {text: "Diagnosticos de registro"}, MessageSectionUpdateEligibility: {text: "Seleccion para actualizar"},
	MessageCommonRetry: {text: "Pulsa r para reintentar."}, MessageCommonBack: {text: "esc volver"}, MessageCommonYes: {text: "si"}, MessageCommonNo: {text: "no"}, MessageCommonAvailable: {text: "disponible"}, MessageCommonUnavailableReason: {text: "no disponible: %s"}, MessageCommonUnknown: {text: "desconocido"}, MessageCommonDangling: {text: "sin referencia"}, MessageCommonUntagged: {text: "<sin etiqueta>"},
	MessageContainersLoading: {text: "Conectando al Docker Engine y cargando contenedores..."}, MessageContainersEmptySummary: {text: "Conexion establecida con Docker Engine, pero no se encontraron contenedores."}, MessageContainersEmptyBody: {text: "No hay contenedores. Crea uno con Docker y pulsa r para reintentar."}, MessageContainersConnectionFailure: {text: "dtop no pudo conectarse a un Docker Engine local compatible."}, MessageContainersConnectionNext: {text: "Comprueba Docker Desktop, docker context, DOCKER_HOST, los permisos del socket o el estado del daemon. Pulsa r para reintentar."}, MessageContainersHelp: {text: "enter acciones | d detalles | s shell | e editar | espacio seleccionar | a seleccionar todo | arriba/abajo mover | o ordenar | r actualizar | izquierda/derecha vista | esc cerrar | q salir"}, MessageDockerHubHelp: {text: "Para obtener actualizaciones de las imagenes de tus contenedores, inicia sesion en Docker Hub con: docker login"}, MessageDockerHubFooter: {text: "Docker Hub: ejecuta docker login para comprobar actualizaciones de forma fiable"},
	MessageImagesLoading: {text: "Cargando imagenes Docker..."}, MessageImagesPartial: {text: "Mostrando las ultimas imagenes conocidas: "}, MessageImagesEmpty: {text: "No se encontraron imagenes Docker."}, MessageImageDetailsTitle: {text: "Detalles de imagen"}, MessageImageDetailsLoad: {text: "Cargando detalles de la imagen..."}, MessageImageDetailsSize: {text: "Tamano: %s"}, MessageImageDetailsCreate: {text: "Creada: %s"}, MessageImageDetailsPlat: {text: "Plataforma: %s"},
	MessageNetworksLoading: {text: "Cargando redes Docker..."}, MessageNetworksPartial: {text: "Mostrando las ultimas redes conocidas: "}, MessageNetworksEmpty: {text: "No se encontraron redes Docker."}, MessageNetworkDetailsTitle: {text: "Detalles de red"}, MessageNetworkDetailsLoad: {text: "Cargando detalles de la red..."},
	MessageVolumesLoading: {text: "Cargando volumenes Docker..."}, MessageVolumesPartial: {text: "Mostrando los ultimos volumenes conocidos: "}, MessageVolumesEmpty: {text: "No se encontraron volumenes Docker."}, MessageVolumeDetailsTitle: {text: "Detalles de volumen"}, MessageVolumeDetailsLoad: {text: "Cargando detalles del volumen..."},
	MessageStacksLoading: {text: "Cargando stacks de Docker Compose..."}, MessageStacksEmpty: {text: "No se encontraron stacks. Solo se pueden descubrir stacks con etiquetas de Docker Compose."}, MessageStackWorkingDirectory: {text: "Directorio de trabajo: %s"}, MessageStackComposeFiles: {text: "Archivos Compose: %s"}, MessageStackDown: {text: "Down: %s"}, MessageStackContainerFallback: {text: "contenedor"}, MessageContainerShellTitle: {text: "Shell del contenedor"}, MessageContainerShellStarting: {text: "Iniciando una shell interactiva..."}, MessageContainerShellFailed: {text: "Fallo la shell del contenedor: "}, MessageContainerShellFailureNext: {text: "Comprueba que el contenedor este en ejecucion, que exista /bin/sh y que los permisos de Docker permitan exec. Pulsa s para reintentar."},
	MessageDetailsID: {text: "ID: %s"}, MessageDetailsImage: {text: "Imagen: %s"}, MessageDetailsState: {text: "Estado: %s"}, MessageDetailsHealth: {text: "Salud: %s"}, MessageDetailsUptime: {text: "Actividad: %s"}, MessageDetailsDriver: {text: "Driver: %s"}, MessageDetailsScope: {text: "Alcance: %s"}, MessageDetailsCreated: {text: "Creado: %s"}, MessageDetailsInternal: {text: "Interna: %s"}, MessageDetailsAttachable: {text: "Conectable: %s"}, MessageDetailsMountpoint: {text: "Punto de montaje: %s"}, MessageContainerDetails: {text: "Detalles del contenedor"}, MessageContainerDetailsLoad: {text: "Cargando detalles del contenedor..."},
	MessageEngineName: {text: "Nombre: %s"}, MessageEngineEndpoint: {text: "Endpoint: %s"}, MessageEngineTransport: {text: "Transporte: %s"}, MessageEngineRemote: {text: "Remoto: %s"}, MessageEngineSecure: {text: "Seguro: %s"}, MessageEngineSource: {text: "Origen: %s"}, MessageEngineServer: {text: "Servidor: %s"}, MessageEngineAPI: {text: "API: %s"}, MessageEngineOS: {text: "SO: %s"}, MessageEngineCPUs: {text: "CPUs: %d"}, MessageEngineRAM: {text: "RAM: %s"},
	MessageLogsLoading: {text: "Cargando el stream de logs..."}, MessageLogsFollowing: {text: "Siguiendo logs en vivo"}, MessageLogsEmpty: {text: "(sin salida de logs)"}, MessageLogsComposeTitle: {text: "Logs de Compose"}, MessageLogsContainerTitle: {text: "Logs del contenedor"}, MessageLogsControls: {text: "arriba/abajo desplazar | esc detener y volver"},
	MessageDockerRemoteUnsupported: {text: "Hay un endpoint Docker remoto configurado, pero D1A solo admite Engines locales. El soporte remoto esta previsto para D1R."}, MessageHeaderConnecting: {text: "conectando con Docker Engine"}, MessageHeaderUnavailable: {text: "Docker no disponible"}, MessageHeaderSummary: {text: "%s %s | CPU %s | RAM %s | %d/%d %s | ORDEN: %s | Docker %s"}, MessageHeaderRunning: {text: "en ejecucion"}, MessageScopeLocal: {text: "LOCAL"}, MessageScopeRemote: {text: "REMOTO"}, MessageSortCPU: {text: "CPU"}, MessageSortMemory: {text: "Memoria"}, MessageSortName: {text: "Nombre"}, MessageSortState: {text: "Estado"},
	MessageColumnName: {text: "NOMBRE"}, MessageColumnState: {text: "ESTADO"}, MessageColumnCPU: {text: "CPU"}, MessageColumnMemory: {text: "MEM"}, MessageColumnHealth: {text: "SALUD"}, MessageColumnUptime: {text: "ACTIV."}, MessageColumnImage: {text: "IMAGEN"}, MessageColumnID: {text: "ID"}, MessageColumnUpdate: {text: "ACTUAL."}, MessageColumnSize: {text: "TAMANO"}, MessageColumnUsed: {text: "USO"}, MessageColumnAge: {text: "EDAD"}, MessageColumnDriver: {text: "DRIVER"}, MessageColumnScope: {text: "ALCANCE"}, MessageColumnServices: {text: "SERVICIOS"}, MessageColumnContainers: {text: "CONTENEDORES"},
	MessageUsageContainers: {one: "%d contenedor", other: "%d contenedores"}, MessageResourceContainers: {one: "%d contenedor", other: "%d contenedores"}, MessageResourceImages: {one: "%d imagen", other: "%d imagenes"}, MessageResourceNetworks: {one: "%d red", other: "%d redes"}, MessageResourceVolumes: {one: "%d volumen", other: "%d volumenes"}, MessageResourceStacks: {one: "%d stack", other: "%d stacks"}, MessageResourceStackContainers: {one: "%d contenedor del stack", other: "%d contenedores del stack"}, MessageResourceStackContainersLabel: {text: "Contenedores del stack"},
	MessageActionSelected: {text: "Seleccion: %s"}, MessageActionControls: {text: "arriba/abajo elegir | enter continuar | esc cancelar"}, MessageActionStop: {text: "Detener"}, MessageActionRestart: {text: "Reiniciar"}, MessageActionForceDelete: {text: "Forzar eliminacion"}, MessageActionDelete: {text: "Eliminar"}, MessageActionDownStack: {text: "Bajar stack"}, MessageActionUpStack: {text: "Levantar stack"}, MessageActionPullUpdate: {text: "Descargar actualizacion"}, MessageActionRecreate: {text: "Recrear contenedores"}, MessageActionUpdateNow: {text: "Actualizar ahora"}, MessageActionApplyUpdate: {text: "Aplicar actualizacion descargada"}, MessageActionEligibility: {text: "%d elegibles, %d omitidos"}, MessageActionCancel: {text: "Cancelar"}, MessageActionTargetUnavailable: {text: " (no disponible: %s)"}, MessageConfirmTitle: {text: "CONFIRMAR: %s"}, MessageConfirmTarget: {text: "Objetivo: %s en %s"}, MessageConfirmMore: {text: "+%d mas"}, MessageConfirmControls: {text: "Estas seguro? [y/N] | esc cancelar"}, MessageResultCompleted: {one: "%d accion completada", other: "%d acciones completadas"}, MessageResultPartial: {text: "%d completadas, %d fallidas"},
	MessageFooterConfirmation: {text: "Confirmacion abajo"}, MessageFooterEdit: {text: "EDITAR: %d seleccionados | espacio alternar | a todos/ninguno | enter acciones | e/esc cancelar"}, MessageFooterStackChild: {text: "s shell  l logs  enter acciones  esc contraer  arriba/abajo seleccionar  r actualizar  izquierda/derecha vistas  q salir"}, MessageFooterStack: {text: "enter expandir  esc contraer  arriba/abajo seleccionar  r actualizar  izquierda/derecha vistas  q salir"}, MessageFooterActions: {text: "arriba/abajo elegir  enter continuar  esc cancelar"}, MessageFooterDetails: {text: "esc volver  izquierda/derecha vistas  q salir"}, MessageFooterResource: {text: "enter acciones  d detalles  e editar  arriba/abajo seleccionar  r actualizar  izquierda/derecha vistas  q salir"}, MessageFooterLogs: {text: "arriba/abajo desplazar  esc detener y volver  q salir"}, MessageFooterContainerDetails: {text: "l logs  esc volver  q salir"}, MessageFooterMinimal: {text: "e editar  q salir  arriba/abajo"}, MessageFooterCompact: {text: "s shell  e editar  arriba/abajo seleccionar  o ordenar  r actualizar  q salir"}, MessageFooterDefault: {text: "enter acciones  d detalles  s shell  l logs  e editar  arriba/abajo seleccionar  o ordenar  r actualizar  izquierda/derecha vistas  ? ayuda  q salir"},
	MessageKeyQuit: {text: "salir"}, MessageKeyNext: {text: "vista siguiente"}, MessageKeyPrevious: {text: "vista anterior"}, MessageKeyRetry: {text: "reintentar"}, MessageKeyUp: {text: "arriba"}, MessageKeyDown: {text: "abajo"}, MessageKeyHelp: {text: "ayuda"}, MessageKeyBack: {text: "volver"}, MessageKeySort: {text: "ordenar"}, MessageKeyEdit: {text: "editar"}, MessageKeyAll: {text: "todo"}, MessageKeySelect: {text: "seleccionar"}, MessageKeyActions: {text: "acciones"}, MessageKeyDetails: {text: "detalles"}, MessageKeyLogs: {text: "logs"}, MessageKeyShell: {text: "shell"},
	MessageStateRunning: {text: "en ejecucion"}, MessageStateStopped: {text: "detenido"}, MessageStateExited: {text: "finalizado"}, MessageStateMixed: {text: "mixto"}, MessageStateDown: {text: "apagado"}, MessageStateMissingComposeFile: {text: "falta archivo Compose"}, MessageHealthHealthy: {text: "saludable"}, MessageHealthUnhealthy: {text: "no saludable"}, MessageHealthStarting: {text: "iniciando"},
}

// Translator is a dependency-free localizer backed by static message maps.
type Translator struct {
	locale   string
	messages map[string]message
}

// New returns a translator for locale. Unsupported locales use English.
func New(locale string) *Translator {
	locale = NormalizeLocale(locale)
	messages := english
	if locale == "es" {
		messages = spanish
	}
	return &Translator{locale: locale, messages: messages}
}

// NewFromEnvironment resolves and constructs the process locale translator.
func NewFromEnvironment() *Translator {
	return New(ResolveLocale())
}

func (translator *Translator) Text(id string, args ...any) string {
	entry := translator.message(id)
	text := entry.text
	if text == "" {
		return id
	}
	return format(text, args...)
}

func (translator *Translator) Plural(id string, count int, args ...any) string {
	entry := translator.message(id)
	text := entry.other
	if count == 1 {
		text = entry.one
	}
	if text == "" {
		return translator.Text(id, args...)
	}
	return format(text, append([]any{count}, args...)...)
}

func (translator *Translator) Decimal(value float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	decimal := strconv.FormatFloat(value, 'f', precision, 64)
	if translator.locale == "es" {
		decimal = strings.Replace(decimal, ".", ",", 1)
	}
	return decimal
}

func (translator *Translator) message(id string) message {
	if translator != nil {
		if entry, ok := translator.messages[id]; ok {
			return entry
		}
	}
	return english[id]
}

func format(message string, args ...any) string {
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

var _ sharedui.Localizer = (*Translator)(nil)
