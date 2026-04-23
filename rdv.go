package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ─── Structures calendrier ────────────────────────────────────────────────────

type RDVCalCard struct {
	RendezVous
	HeureH string // "08", "09", ... pour le placement dans la grille
}

type WeekDay struct {
	Date    string // YYYY-MM-DD
	DateFR  string // "14"
	MoisFR  string // "Avr"
	JourFR  string // "Dim", "Lun", ...
	IsToday bool
	Cards   []RDVCalCard
}

var moisFRShort = []string{"", "Jan", "Fév", "Mar", "Avr", "Mai", "Jun", "Jul", "Aoû", "Sep", "Oct", "Nov", "Déc"}

// Algérie : semaine commence le dimanche
var joursFRShort = []string{"Dim", "Lun", "Mar", "Mer", "Jeu", "Ven", "Sam"}

func buildCalCards(rdvs []RendezVous) []RDVCalCard {
	cards := make([]RDVCalCard, 0, len(rdvs))
	for _, rdv := range rdvs {
		parts := strings.Split(rdv.HeureRDV, ":")
		h, _ := strconv.Atoi(parts[0])
		cards = append(cards, RDVCalCard{
			RendezVous: rdv,
			HeureH:     fmt.Sprintf("%02d", h),
		})
	}
	return cards
}

// weekStartOf retourne le dimanche de la semaine contenant t (semaine DIM→SAM).
func weekStartOf(t time.Time) time.Time {
	wd := int(t.Weekday()) // 0=dim, 1=lun, ..., 6=sam
	d := t.AddDate(0, 0, -wd)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local)
}

// heureToMin convertit "HH:MM" en minutes depuis minuit.
func heureToMin(h string) int {
	parts := strings.Split(h, ":")
	hh, _ := strconv.Atoi(parts[0])
	mm := 0
	if len(parts) > 1 {
		mm, _ = strconv.Atoi(parts[1])
	}
	return hh*60 + mm
}

// rdvSlotOccupe vérifie si un nouveau RDV chevauche un existant sur la même journée.
func rdvSlotOccupe(dateRDV, heureRDV string, dureeMin int) bool {
	rdvs, err := getRDVsByPeriod(dateRDV, dateRDV)
	if err != nil {
		return false
	}
	newStart := heureToMin(heureRDV)
	newEnd := newStart + dureeMin
	for _, rdv := range rdvs {
		if rdv.Statut == "annule" {
			continue
		}
		existStart := heureToMin(rdv.HeureRDV)
		existEnd := existStart + rdv.DureeMin
		if newStart < existEnd && newEnd > existStart {
			return true
		}
	}
	return false
}

func buildHeures(ouverture, fermeture int) (heures []string, creneaux []string) {
	for h := ouverture; h <= fermeture; h++ {
		heures = append(heures, fmt.Sprintf("%02d", h))
		creneaux = append(creneaux, fmt.Sprintf("%02d:00", h))
		if h < fermeture {
			creneaux = append(creneaux, fmt.Sprintf("%02d:30", h))
		}
	}
	return
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func rdvListHandler(w http.ResponseWriter, r *http.Request) {
	vue := r.URL.Query().Get("vue")
	if vue == "" {
		vue = "calendrier"
	}

	// Début de semaine (dimanche)
	semParam := r.URL.Query().Get("semaine")
	var sunday time.Time
	if semParam != "" {
		t, err := time.ParseInLocation("2006-01-02", semParam, time.Local)
		if err == nil {
			sunday = weekStartOf(t)
		}
	}
	if sunday.IsZero() {
		sunday = weekStartOf(time.Now())
	}

	today := time.Now().Format("2006-01-02")
	prevSunday := sunday.AddDate(0, 0, -7).Format("2006-01-02")
	nextSunday := sunday.AddDate(0, 0, 7).Format("2006-01-02")
	saturday := sunday.AddDate(0, 0, 6)

	// Titre de la semaine
	semaineTitre := fmt.Sprintf("%d – %d %s %d",
		sunday.Day(), saturday.Day(),
		moisFRShort[int(saturday.Month())], saturday.Year())
	if sunday.Month() != saturday.Month() {
		semaineTitre = fmt.Sprintf("%d %s – %d %s %d",
			sunday.Day(), moisFRShort[int(sunday.Month())],
			saturday.Day(), moisFRShort[int(saturday.Month())], saturday.Year())
	}

	// 7 jours Dim→Sam
	weekDays := make([]WeekDay, 7)
	for i := 0; i < 7; i++ {
		d := sunday.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		weekDays[i] = WeekDay{
			Date:    dateStr,
			DateFR:  fmt.Sprintf("%d", d.Day()),
			MoisFR:  moisFRShort[int(d.Month())],
			JourFR:  joursFRShort[i],
			IsToday: dateStr == today,
		}
	}

	// RDVs de la semaine
	weekRDVs, _ := getRDVsByPeriod(sunday.Format("2006-01-02"), saturday.Format("2006-01-02"))
	byDate := map[string][]RendezVous{}
	for _, rdv := range weekRDVs {
		byDate[rdv.DateRDV] = append(byDate[rdv.DateRDV], rdv)
	}
	for i := range weekDays {
		weekDays[i].Cards = buildCalCards(byDate[weekDays[i].Date])
	}

	filtreStatut := r.URL.Query().Get("statut")
	allRDVs, _ := getRDVsByStatut(filtreStatut)
	clients, _ := getClients()
	vehicules, _ := getVehicules()

	gs := getSettings()
	hOuv, _ := strconv.Atoi(gs["heure_ouverture"])
	hFer, _ := strconv.Atoi(gs["heure_fermeture"])
	if hOuv == 0 {
		hOuv = 8
	}
	if hFer == 0 {
		hFer = 18
	}
	heures, creneaux := buildHeures(hOuv, hFer)

	type RDVData struct {
		Vue          string
		Semaine      string
		SemainePrev  string
		SemaineNext  string
		SemaineTitre string
		WeekDays     []WeekDay
		AllRDVs      []RendezVous
		FiltreStatut string
		Clients      []Client
		Vehicules    []Vehicule
		Heures       []string
		Creneaux     []string
	}

	renderPage(w, "rdv_list", PageData{
		Title:      "Rendez-vous",
		ActivePage: "rdv",
		Session:    getSession(r),
		Data: RDVData{
			Vue:          vue,
			Semaine:      sunday.Format("2006-01-02"),
			SemainePrev:  prevSunday,
			SemaineNext:  nextSunday,
			SemaineTitre: semaineTitre,
			WeekDays:     weekDays,
			AllRDVs:      allRDVs,
			FiltreStatut: filtreStatut,
			Clients:      clients,
			Vehicules:    vehicules,
			Heures:       heures,
			Creneaux:     creneaux,
		},
		Success: r.URL.Query().Get("ok"),
		Error:   r.URL.Query().Get("err"),
	})
}

func rdvCreateHandler(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	vehiculeID, _ := strconv.ParseInt(r.FormValue("vehicule_id"), 10, 64)
	dureeMin, _ := strconv.Atoi(r.FormValue("duree_min"))
	dateRDV := r.FormValue("date_rdv")
	heureRDV := r.FormValue("heure_rdv")
	motif := strings.TrimSpace(r.FormValue("motif"))

	if clientID == 0 || vehiculeID == 0 || dateRDV == "" || heureRDV == "" || motif == "" {
		http.Redirect(w, r, "/rdv?err=Champs+obligatoires+manquants", http.StatusFound)
		return
	}
	if dureeMin <= 0 {
		dureeMin = 60
	}

	if rdvSlotOccupe(dateRDV, heureRDV, dureeMin) {
		http.Redirect(w, r, "/rdv?err=Ce+créneau+est+déjà+occupé+—+choisissez+une+autre+heure", http.StatusFound)
		return
	}

	_, err := createRDV(clientID, vehiculeID, dateRDV, heureRDV, dureeMin, motif, r.FormValue("notes"))
	if err != nil {
		log.Println("createRDV:", err)
		http.Redirect(w, r, "/rdv?err=Erreur+lors+de+la+création+du+rendez-vous", http.StatusFound)
		return
	}
	// Revenir à la semaine courante (pas la semaine du RDV)
	http.Redirect(w, r, "/rdv?vue=calendrier&ok=RDV+créé", http.StatusFound)
}

func rdvViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	rdv, err := getRDVByID(id)
	if err != nil {
		http.Redirect(w, r, "/rdv?err=RDV+introuvable", http.StatusFound)
		return
	}
	renderPage(w, "rdv_view", PageData{
		Title:      "RDV — " + rdv.ClientNom,
		ActivePage: "rdv",
		Session:    getSession(r),
		Data:       rdv,
		Success:    r.URL.Query().Get("ok"),
		Error:      r.URL.Query().Get("err"),
	})
}

func rdvStatutHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	statut := r.FormValue("statut")
	allowed := map[string]bool{
		"planifie": true, "confirme": true, "annule": true,
	}
	if !allowed[statut] {
		http.Error(w, "Statut invalide", http.StatusBadRequest)
		return
	}
	updateRDVStatut(id, statut) //nolint:errcheck
	http.Redirect(w, r, fmt.Sprintf("/rdv/view?id=%d&ok=Statut+mis+à+jour", id), http.StatusFound)
}

func rdvDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	deleteRDV(id) //nolint:errcheck
	http.Redirect(w, r, "/rdv?ok=RDV+supprimé", http.StatusFound)
}
