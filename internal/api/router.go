package api

import (
	"net/http"
	"strings"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Providers
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			HandleListProviders(w, r)
		case http.MethodPost:
			HandleCreateProvider(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/providers/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			HandleUpdateProvider(w, r)
		case http.MethodDelete:
			HandleDeleteProvider(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Personas
	mux.HandleFunc("/api/personas", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			HandleListPersonas(w, r)
		case http.MethodPost:
			HandleCreatePersona(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/personas/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			HandleUpdatePersona(w, r)
		case http.MethodDelete:
			HandleDeletePersona(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Folders
	mux.HandleFunc("/api/folders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			HandleListFolders(w, r)
		case http.MethodPost:
			HandleCreateFolder(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/folders/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			HandleUpdateFolder(w, r)
		case http.MethodDelete:
			HandleDeleteFolder(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Templates
	mux.HandleFunc("/api/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			HandleListTemplates(w, r)
		case http.MethodPost:
			HandleCreateTemplate(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/templates/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			HandleUpdateTemplate(w, r)
		case http.MethodDelete:
			HandleDeleteTemplate(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Rooms
	mux.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			HandleListRooms(w, r)
		case http.MethodPost:
			HandleCreateRoom(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/rooms/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				HandleGetRoom(w, r)
			case http.MethodPut:
				HandleUpdateRoom(w, r)
			case http.MethodDelete:
				HandleDeleteRoom(w, r)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}

		if len(parts) >= 2 {
			switch parts[1] {
			case "agents":
				if len(parts) == 2 && r.Method == http.MethodPost {
					HandleAddAgent(w, r)
				} else if len(parts) == 3 && r.Method == http.MethodDelete {
					HandleRemoveAgent(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "start":
				if r.Method == http.MethodPost {
					HandleStartRoom(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "stop":
				if r.Method == http.MethodPost {
					HandleStopRoom(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "restart":
				if r.Method == http.MethodPost {
					HandleRestartRoom(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "messages":
				if r.Method == http.MethodGet {
					HandleGetMessages(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "syscmd":
				if r.Method == http.MethodPost {
					HandleSystemCommand(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "observer-chat":
				if r.Method == http.MethodPost {
					HandleObserverChat(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "clone":
				if r.Method == http.MethodPost {
					HandleCloneRoom(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "export":
				if r.Method == http.MethodGet {
					HandleExportRoom(w, r)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case "ws":
				HandleWebSocket(w, r)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})

	return mux
}
