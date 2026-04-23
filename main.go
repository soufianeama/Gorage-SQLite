package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html
var tmplFS embed.FS

//go:embed static
var staticFS embed.FS

// ─── Données communes de page ────────────────────────────────────────────────

type PageData struct {
	Title      string
	ActivePage string
	Session    *Session
	Data       interface{}
	Error      string
	Success    string
	CSRFToken  string
}

// ─── Fonctions de template ───────────────────────────────────────────────────

var funcMap = template.FuncMap{
	// Date du jour au format YYYY-MM-DD (pour les champs <input type="date">)
	"today": func() string {
		return time.Now().Format("2006-01-02")
	},
	// Convertit YYYY-MM-DD → JJ/MM/AAAA
	"dateFR": func(s string) string {
		if len(s) < 10 {
			return s
		}
		return s[8:10] + "/" + s[5:7] + "/" + s[0:4]
	},
	// Initiale en majuscule pour l'avatar
	"initial": func(s string) string {
		if s == "" {
			return "?"
		}
		return strings.ToUpper(s[:1])
	},
	// Libellé lisible du statut
	"statutLabel": func(s string) string {
		switch s {
		case "en_cours":
			return "En cours"
		case "attente_pieces":
			return "Attente pièces"
		case "termine":
			return "Terminé"
		case "livre":
			return "Livré"
		default:
			return s
		}
	},
	// Classe CSS du badge de statut
	"statutClass": func(s string) string {
		switch s {
		case "en_cours":
			return "badge-warning"
		case "attente_pieces":
			return "badge-info"
		case "termine":
			return "badge-success"
		case "livre":
			return "badge-secondary"
		default:
			return "badge-secondary"
		}
	},
	// Formatage monnaie DZD
	"dzd": func(v float64) string {
		return fmt.Sprintf("%.2f DZD", v)
	},
	// Libellé du type de ligne devis/facture
	"ligneTypeLabel": func(t string) string {
		switch t {
		case "main_oeuvre":
			return "Main d'œuvre"
		case "piece":
			return "Pièce"
		case "fourniture":
			return "Fourniture"
		default:
			return "Autre"
		}
	},
	// Crée un slice de strings pour les boucles {{range $s := slice "a" "b"}}
	"slice": func(args ...string) []string {
		return args
	},
	// Libellé du statut RDV
	"rdvStatutLabel": func(s string) string {
		switch s {
		case "planifie":
			return "Planifié"
		case "confirme":
			return "Confirmé"
		case "annule":
			return "Annulé"
		default:
			return s
		}
	},
	// Classe CSS badge RDV
	"rdvStatutClass": func(s string) string {
		switch s {
		case "planifie":
			return "badge-secondary"
		case "confirme":
			return "badge-info"
		case "annule":
			return "badge-danger"
		default:
			return "badge-secondary"
		}
	},
}

func parseFloatDef(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return def
}

// renderPage : parse base.html + la page spécifique et exécute "base"
func renderPage(w http.ResponseWriter, page string, data PageData) {
	if data.Session != nil {
		data.CSRFToken = data.Session.CSRFToken
	}
	t, err := template.New("").Funcs(funcMap).ParseFS(tmplFS,
		"templates/base.html",
		"templates/"+page+".html",
	)
	if err != nil {
		log.Println("Erreur template:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err = t.ExecuteTemplate(w, "base", data); err != nil {
		log.Println("Erreur rendu template :", err)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.New("").Funcs(funcMap).ParseFS(tmplFS, "templates/404.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	t.ExecuteTemplate(w, "404", nil) //nolint:errcheck
}

// renderAuth : pages login/setup (sans sidebar)
func renderAuth(w http.ResponseWriter, page string, data PageData) {
	t, err := template.New("").Funcs(funcMap).ParseFS(tmplFS,
		"templates/"+page+".html",
	)
	if err != nil {
		log.Println("Erreur template auth:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err = t.ExecuteTemplate(w, page, data); err != nil {
		log.Println("Erreur rendu template auth :", err)
	}
}

// ─── main ────────────────────────────────────────────────────────────────────

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	// Fichiers statiques (CSS, JS)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))

	// Routes publiques (setup / login)
	mux.HandleFunc("GET /setup", setupGetHandler)
	mux.HandleFunc("POST /setup", setupPostHandler)
	mux.HandleFunc("GET /login", loginGetHandler)
	mux.HandleFunc("POST /login", loginPostHandler)
	mux.HandleFunc("GET /logout", logoutHandler)

	// Tableau de bord
	mux.HandleFunc("GET /", authMiddleware(dashboardHandler))

	// Clients
	mux.HandleFunc("GET /clients", authMiddleware(clientsListHandler))
	mux.HandleFunc("POST /clients", authMiddleware(clientsCreateHandler))
	mux.HandleFunc("GET /clients/edit", authMiddleware(clientsEditGetHandler))
	mux.HandleFunc("POST /clients/edit", authMiddleware(clientsEditPostHandler))
	mux.HandleFunc("POST /clients/delete", authMiddleware(clientsDeleteHandler))

	// Véhicules
	mux.HandleFunc("GET /vehicules", authMiddleware(vehiculesListHandler))
	mux.HandleFunc("POST /vehicules", authMiddleware(vehiculesCreateHandler))
	mux.HandleFunc("GET /vehicules/view", authMiddleware(vehiculeViewHandler))
	mux.HandleFunc("GET /vehicules/edit", authMiddleware(vehiculesEditGetHandler))
	mux.HandleFunc("POST /vehicules/edit", authMiddleware(vehiculesEditPostHandler))
	mux.HandleFunc("POST /vehicules/delete", authMiddleware(vehiculesDeleteHandler))

	// Interventions
	mux.HandleFunc("GET /interventions", authMiddleware(interventionsListHandler))
	mux.HandleFunc("POST /interventions", authMiddleware(interventionsCreateHandler))
	mux.HandleFunc("GET /interventions/view", authMiddleware(interventionViewHandler))
	mux.HandleFunc("POST /interventions/update", authMiddleware(interventionUpdateHandler))
	mux.HandleFunc("POST /interventions/statut", authMiddleware(interventionStatutHandler))
	mux.HandleFunc("POST /interventions/delete", authMiddleware(interventionDeleteHandler))
	mux.HandleFunc("GET /interventions/facture", authMiddleware(factureHandler))
	mux.HandleFunc("GET /interventions/accueil", authMiddleware(ficheAccueilHandler))

	// Devis / OR (admin uniquement)
	mux.HandleFunc("GET /devis", adminMiddleware(devisListHandler))
	mux.HandleFunc("GET /devis/new", adminMiddleware(devisNewHandler))
	mux.HandleFunc("POST /devis", adminMiddleware(devisCreateHandler))
	mux.HandleFunc("GET /devis/view", adminMiddleware(devisViewHandler))
	mux.HandleFunc("POST /devis/statut", adminMiddleware(devisStatutHandler))
	mux.HandleFunc("POST /devis/delete", adminMiddleware(devisDeleteHandler))
	mux.HandleFunc("POST /devis/facturer", adminMiddleware(devisFacturerHandler))
	mux.HandleFunc("GET /devis/print", adminMiddleware(devisPrintHandler))
	mux.HandleFunc("POST /devis/email", adminMiddleware(devisEmailHandler))

	// Factures officielles (admin uniquement)
	mux.HandleFunc("GET /factures", adminMiddleware(facturesListHandler))
	mux.HandleFunc("GET /factures/view", adminMiddleware(factureViewHandler))
	mux.HandleFunc("POST /factures/payer", adminMiddleware(facturePayerHandler))
	mux.HandleFunc("POST /factures/unpayer", adminMiddleware(factureUnpayerHandler))
	mux.HandleFunc("POST /factures/annuler", adminMiddleware(factureAnnulerHandler))
	mux.HandleFunc("GET /factures/nouvelle", adminMiddleware(factureNouvelleFormHandler))
	mux.HandleFunc("POST /factures/nouvelle", adminMiddleware(factureDirecteHandler))
	mux.HandleFunc("GET /factures/print", adminMiddleware(factureLegalePrintHandler))
	mux.HandleFunc("POST /factures/avoir", adminMiddleware(avoirHandler))
	mux.HandleFunc("POST /factures/email", adminMiddleware(factureEmailHandler))

	// Rendez-vous
	mux.HandleFunc("GET /rdv", authMiddleware(rdvListHandler))
	mux.HandleFunc("POST /rdv", authMiddleware(rdvCreateHandler))
	mux.HandleFunc("GET /rdv/view", authMiddleware(rdvViewHandler))
	mux.HandleFunc("POST /rdv/statut", authMiddleware(rdvStatutHandler))
	mux.HandleFunc("POST /rdv/delete", authMiddleware(rdvDeleteHandler))

	// Stock pièces (admin uniquement)
	mux.HandleFunc("GET /stock", adminMiddleware(stockListHandler))
	mux.HandleFunc("POST /stock", adminMiddleware(stockCreateHandler))
	mux.HandleFunc("GET /stock/edit", adminMiddleware(stockEditGetHandler))
	mux.HandleFunc("POST /stock/edit", adminMiddleware(stockEditPostHandler))
	mux.HandleFunc("POST /stock/ajuster", adminMiddleware(stockAjusterHandler))
	mux.HandleFunc("POST /stock/delete", adminMiddleware(stockDeleteHandler))

	// Recherche globale
	mux.HandleFunc("GET /search", authMiddleware(searchHandler))

	// Statistiques (admin uniquement)
	mux.HandleFunc("GET /stats", adminMiddleware(statsHandler))

	// Sauvegarde (admin uniquement)
	mux.HandleFunc("GET /backup", adminMiddleware(backupHandler))
	mux.HandleFunc("POST /backup/cloud", adminMiddleware(cloudBackupHandler))

	// Paramètres (admin uniquement, sauf changement de mot de passe)
	mux.HandleFunc("GET /settings", adminMiddleware(settingsGetHandler))
	mux.HandleFunc("POST /settings", adminMiddleware(settingsPostHandler))
	mux.HandleFunc("POST /settings/password", authMiddleware(changePasswordHandler))
	mux.HandleFunc("POST /settings/users/create", adminMiddleware(userCreateHandler))
	mux.HandleFunc("POST /settings/users/delete", adminMiddleware(userDeleteHandler))

	// Catch-all : 404 personnalisée
	mux.HandleFunc("/", notFoundHandler)

	addr := "127.0.0.1:8585"
	fmt.Printf("\n╔══════════════════════════════════════╗\n")
	fmt.Printf("║    🔧  Gorage — v1.0.0               ║\n")
	fmt.Printf("║    http://%s              ║\n", addr)
	fmt.Printf("╚══════════════════════════════════════╝\n\n")

	go openBrowser("http://" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}
