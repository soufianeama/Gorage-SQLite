package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ─── Setup ───────────────────────────────────────────────────────────────────

func setupGetHandler(w http.ResponseWriter, r *http.Request) {
	if userExists() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	renderAuth(w, "setup", PageData{Title: "Configuration initiale"})
}

func setupPostHandler(w http.ResponseWriter, r *http.Request) {
	if userExists() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	data := PageData{Title: "Configuration initiale"}

	if username == "" || password == "" {
		data.Error = "Le nom d'utilisateur et le mot de passe sont obligatoires."
		renderAuth(w, "setup", data)
		return
	}
	if len(password) < 6 {
		data.Error = "Le mot de passe doit contenir au moins 6 caractères."
		renderAuth(w, "setup", data)
		return
	}
	if password != confirm {
		data.Error = "Les mots de passe ne correspondent pas."
		renderAuth(w, "setup", data)
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		data.Error = "Erreur lors du hachage du mot de passe."
		renderAuth(w, "setup", data)
		return
	}
	if err = createUser(username, hash); err != nil {
		log.Println("createUser:", err)
		data.Error = "Erreur lors de la création du compte."
		renderAuth(w, "setup", data)
		return
	}
	http.Redirect(w, r, "/login?setup=ok", http.StatusFound)
}

// ─── Login / Logout ──────────────────────────────────────────────────────────

func loginGetHandler(w http.ResponseWriter, r *http.Request) {
	if !userExists() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if getSession(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	data := PageData{Title: "Connexion"}
	if r.URL.Query().Get("setup") == "ok" {
		data.Success = "Compte créé avec succès. Connectez-vous."
	}
	renderAuth(w, "login", data)
}

func loginPostHandler(w http.ResponseWriter, r *http.Request) {
	if !userExists() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	ip := loginIP(r)
	data := PageData{Title: "Connexion"}

	if isLoginBlocked(ip) {
		data.Error = "Trop de tentatives échouées. Réessayez dans 15 minutes."
		renderAuth(w, "login", data)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	id, hash, role, err := getUserByUsername(username)
	if err != nil || !checkPassword(password, hash) {
		recordLoginFailure(ip)
		data.Error = "Nom d'utilisateur ou mot de passe incorrect."
		renderAuth(w, "login", data)
		return
	}

	resetLoginAttempts(ip)
	sid, err := createSession(id, username, role)
	if err != nil {
		data.Error = "Erreur lors de la création de la session."
		renderAuth(w, "login", data)
		return
	}
	setSessionCookie(w, sid)
	http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	destroySession(r)
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ─── Tableau de bord ─────────────────────────────────────────────────────────

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		notFoundHandler(w, r)
		return
	}
	interventions, _ := getInterventions("en_cours")
	if len(interventions) > 8 {
		interventions = interventions[:8]
	}
	rdvsAujourdhui, _ := getRDVsAujourdhui()
	piecesStockBas, _ := getPieces("", "")
	var stockBas []Piece
	for _, p := range piecesStockBas {
		if p.StockBas {
			stockBas = append(stockBas, p)
		}
	}

	type DashData struct {
		Stats          Stats
		Interventions  []Intervention
		RDVsAujourdhui []RendezVous
		StockBas       []Piece
	}

	renderPage(w, "dashboard", PageData{
		Title:      "Tableau de bord",
		ActivePage: "dashboard",
		Session:    getSession(r),
		Data: DashData{
			Stats:          getStats(),
			Interventions:  interventions,
			RDVsAujourdhui: rdvsAujourdhui,
			StockBas:       stockBas,
		},
	})
}

// ─── Clients ─────────────────────────────────────────────────────────────────

func clientsListHandler(w http.ResponseWriter, r *http.Request) {
	clients, err := getClients()
	if err != nil {
		http.Error(w, "Erreur base de données", http.StatusInternalServerError)
		return
	}
	renderPage(w, "clients", PageData{
		Title:      "Clients",
		ActivePage: "clients",
		Session:    getSession(r),
		Data:       clients,
		Success:    r.URL.Query().Get("ok"),
		Error:      r.URL.Query().Get("err"),
	})
}

func clientsCreateHandler(w http.ResponseWriter, r *http.Request) {
	nom := strings.TrimSpace(r.FormValue("nom"))
	prenom := strings.TrimSpace(r.FormValue("prenom"))
	if nom == "" || prenom == "" {
		http.Redirect(w, r, "/clients?err=Nom+et+prénom+obligatoires", http.StatusFound)
		return
	}
	_, err := createClient(nom, prenom,
		r.FormValue("telephone"),
		r.FormValue("email"),
		r.FormValue("adresse"),
	)
	if err != nil {
		log.Println("createClient:", err)
		http.Redirect(w, r, "/clients?err=Erreur+lors+de+la+création+du+client", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/clients?ok=Client+créé+avec+succès", http.StatusFound)
}

func clientsEditGetHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	client, err := getClientByID(id)
	if err != nil {
		http.Redirect(w, r, "/clients?err=Client+introuvable", http.StatusFound)
		return
	}
	renderPage(w, "client_edit", PageData{
		Title:      "Modifier client",
		ActivePage: "clients",
		Session:    getSession(r),
		Data:       client,
	})
}

func clientsEditPostHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	nom := strings.TrimSpace(r.FormValue("nom"))
	prenom := strings.TrimSpace(r.FormValue("prenom"))
	if nom == "" || prenom == "" {
		http.Redirect(w, r, "/clients?err=Nom+et+prénom+obligatoires", http.StatusFound)
		return
	}
	if err := updateClient(id, nom, prenom,
		r.FormValue("telephone"),
		r.FormValue("email"),
		r.FormValue("adresse"),
	); err != nil {
		log.Println("updateClient:", err)
		http.Redirect(w, r, "/clients?err=Erreur+lors+de+la+mise+à+jour", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/clients?ok=Client+mis+à+jour", http.StatusFound)
}

func clientsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err := deleteClient(id); err != nil {
		http.Redirect(w, r, "/clients?err=Suppression+impossible+(véhicules+liés?)", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/clients?ok=Client+supprimé", http.StatusFound)
}

// ─── Véhicules ───────────────────────────────────────────────────────────────

func vehiculesListHandler(w http.ResponseWriter, r *http.Request) {
	vehicules, _ := getVehicules()
	clients, _ := getClients()

	type VehData struct {
		Vehicules []Vehicule
		Clients   []Client
	}
	renderPage(w, "vehicules", PageData{
		Title:      "Véhicules",
		ActivePage: "vehicules",
		Session:    getSession(r),
		Data:       VehData{Vehicules: vehicules, Clients: clients},
		Success:    r.URL.Query().Get("ok"),
		Error:      r.URL.Query().Get("err"),
	})
}

func vehiculesCreateHandler(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	annee, _ := strconv.ParseInt(r.FormValue("annee"), 10, 64)
	km, _ := strconv.ParseInt(r.FormValue("kilometrage"), 10, 64)
	immat := strings.TrimSpace(r.FormValue("immatriculation"))
	marque := strings.TrimSpace(r.FormValue("marque"))
	modele := strings.TrimSpace(r.FormValue("modele"))

	if clientID == 0 || marque == "" || modele == "" || immat == "" {
		http.Redirect(w, r, "/vehicules?err=Champs+obligatoires+manquants", http.StatusFound)
		return
	}
	_, err := createVehicule(clientID, marque, modele, annee, immat, r.FormValue("vin"), km)
	if err != nil {
		log.Println("createVehicule:", err)
		http.Redirect(w, r, "/vehicules?err=Erreur+lors+de+l'ajout+du+véhicule", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/vehicules?ok=Véhicule+ajouté", http.StatusFound)
}

func vehiculeViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	v, err := getVehiculeByID(id)
	if err != nil {
		http.Redirect(w, r, "/vehicules?err=Véhicule+introuvable", http.StatusFound)
		return
	}
	interventions, _ := getInterventionsByVehicule(id)
	devis, _ := getDevisByVehicule(id)
	rdvs, _ := getRDVsByVehicule(id)

	type VehiculeViewData struct {
		Vehicule      Vehicule
		Interventions []Intervention
		Devis         []Devis
		RDVs          []RendezVous
	}
	renderPage(w, "vehicule_view", PageData{
		Title:      "Fiche — " + v.Immatriculation,
		ActivePage: "vehicules",
		Session:    getSession(r),
		Data: VehiculeViewData{
			Vehicule:      v,
			Interventions: interventions,
			Devis:         devis,
			RDVs:          rdvs,
		},
	})
}

func vehiculesEditGetHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	v, err := getVehiculeByID(id)
	if err != nil {
		http.Redirect(w, r, "/vehicules?err=Véhicule+introuvable", http.StatusFound)
		return
	}
	clients, _ := getClients()

	type EditData struct {
		Vehicule Vehicule
		Clients  []Client
	}
	renderPage(w, "vehicule_edit", PageData{
		Title:      "Modifier véhicule",
		ActivePage: "vehicules",
		Session:    getSession(r),
		Data:       EditData{Vehicule: v, Clients: clients},
	})
}

func vehiculesEditPostHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	annee, _ := strconv.ParseInt(r.FormValue("annee"), 10, 64)
	km, _ := strconv.ParseInt(r.FormValue("kilometrage"), 10, 64)
	immat := strings.TrimSpace(r.FormValue("immatriculation"))
	marque := strings.TrimSpace(r.FormValue("marque"))
	modele := strings.TrimSpace(r.FormValue("modele"))

	if clientID == 0 || marque == "" || modele == "" || immat == "" {
		http.Redirect(w, r, fmt.Sprintf("/vehicules/edit?id=%d&err=Champs+obligatoires+manquants", id), http.StatusFound)
		return
	}
	if err := updateVehicule(id, clientID, marque, modele, annee, immat, r.FormValue("vin"), km); err != nil {
		log.Println("updateVehicule:", err)
		http.Redirect(w, r, fmt.Sprintf("/vehicules/edit?id=%d&err=Erreur+lors+de+la+mise+à+jour", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/vehicules?ok=Véhicule+mis+à+jour", http.StatusFound)
}

func vehiculesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err := deleteVehicule(id); err != nil {
		http.Redirect(w, r, "/vehicules?err=Suppression+impossible", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/vehicules?ok=Véhicule+supprimé", http.StatusFound)
}

// ─── Interventions ───────────────────────────────────────────────────────────

func interventionsListHandler(w http.ResponseWriter, r *http.Request) {
	enCours, _       := getInterventions("en_cours")
	attentePieces, _ := getInterventions("attente_pieces")
	termine, _       := getInterventions("termine")
	livre, _         := getInterventions("livre")
	vehicules, _     := getVehicules()

	type KanbanData struct {
		EnCours       []Intervention
		AttentePieces []Intervention
		Termine       []Intervention
		Livre         []Intervention
		Vehicules     []Vehicule
	}
	renderPage(w, "interventions", PageData{
		Title:      "Interventions",
		ActivePage: "interventions",
		Session:    getSession(r),
		Data: KanbanData{
			EnCours:       enCours,
			AttentePieces: attentePieces,
			Termine:       termine,
			Livre:         livre,
			Vehicules:     vehicules,
		},
		Success: r.URL.Query().Get("ok"),
		Error:   r.URL.Query().Get("err"),
	})
}

func interventionsCreateHandler(w http.ResponseWriter, r *http.Request) {
	vehiculeID, _ := strconv.ParseInt(r.FormValue("vehicule_id"), 10, 64)
	desc := strings.TrimSpace(r.FormValue("description"))
	dateEntree := r.FormValue("date_entree")

	if vehiculeID == 0 || desc == "" || dateEntree == "" {
		http.Redirect(w, r, "/interventions?err=Champs+obligatoires+manquants", http.StatusFound)
		return
	}
	id, err := createIntervention(vehiculeID, dateEntree, desc, 0, 0)
	if err != nil {
		log.Println("createIntervention:", err)
		http.Redirect(w, r, "/interventions?err=Erreur+lors+de+la+création", http.StatusFound)
		return
	}
	// Placer dans la colonne demandée (si différent du défaut en_cours)
	allowed := map[string]bool{
		"en_cours": true, "attente_pieces": true, "termine": true, "livre": true,
	}
	if s := r.FormValue("statut_initial"); allowed[s] && s != "en_cours" {
		updateStatutIntervention(id, s) //nolint:errcheck
	}
	http.Redirect(w, r, "/interventions?ok=Intervention+créée", http.StatusFound)
}

func interventionViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	inv, err := getInterventionByID(id)
	if err != nil {
		http.Redirect(w, r, "/interventions?err=Intervention+introuvable", http.StatusFound)
		return
	}
	renderPage(w, "intervention_detail", PageData{
		Title:      "Intervention #" + strconv.FormatInt(id, 10),
		ActivePage: "interventions",
		Session:    getSession(r),
		Data:       inv,
		Success:    r.URL.Query().Get("ok"),
		Error:      r.URL.Query().Get("err"),
	})
}

func interventionUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)

	err := updateIntervention(id,
		r.FormValue("description"),
		r.FormValue("diagnostic"),
		r.FormValue("travaux_effectues"),
		r.FormValue("pieces_utilisees"),
		0, 0,
		r.FormValue("statut"),
	)
	if err != nil {
		log.Println("updateIntervention:", err)
		http.Redirect(w, r, "/interventions/view?id="+strconv.FormatInt(id, 10)+"&err=Erreur+lors+de+la+sauvegarde", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/interventions/view?id="+strconv.FormatInt(id, 10)+"&ok=Sauvegardé", http.StatusFound)
}

// interventionStatutHandler : mise à jour rapide du statut (HTMX ou formulaire)
func interventionStatutHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	statut := r.FormValue("statut")
	allowed := map[string]bool{
		"en_cours": true, "attente_pieces": true, "termine": true, "livre": true,
	}
	if !allowed[statut] {
		http.Error(w, "Statut invalide", http.StatusBadRequest)
		return
	}
	updateStatutIntervention(id, statut) //nolint:errcheck

	// Réponse HTMX : retourner le badge mis à jour
	if r.Header.Get("HX-Request") == "true" {
		label := map[string]string{
			"en_cours": "En cours", "attente_pieces": "Attente pièces",
			"termine": "Terminé", "livre": "Livré",
		}[statut]
		cls := map[string]string{
			"en_cours": "badge-warning", "attente_pieces": "badge-info",
			"termine": "badge-success", "livre": "badge-secondary",
		}[statut]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<span class="badge ` + cls + `">` + label + `</span>`)) //nolint:errcheck
		return
	}
	http.Redirect(w, r, "/interventions", http.StatusFound)
}

func interventionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	deleteIntervention(id) //nolint:errcheck
	http.Redirect(w, r, "/interventions?ok=Intervention+supprimée", http.StatusFound)
}

// ─── Sauvegarde ──────────────────────────────────────────────────────────────

func statsHandler(w http.ResponseWriter, r *http.Request) {
	type StatsData struct {
		Interventions []MoisStat
		Pieces        []PieceStat
	}
	renderPage(w, "stats", PageData{
		Title:      "Statistiques",
		ActivePage: "stats",
		Session:    getSession(r),
		Data: StatsData{
			Interventions: getStatsInterventionsParMois(),
			Pieces:        getStatsPiecesTop(),
		},
	})
}

func backupHandler(w http.ResponseWriter, r *http.Request) {
	tmp := fmt.Sprintf("/tmp/gorage-backup-%s.db", time.Now().Format("2006-01-02-150405"))
	if _, err := db.Exec("VACUUM INTO ?", tmp); err != nil {
		log.Println("backup:", err)
		http.Error(w, "Erreur lors de la sauvegarde.", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp)

	filename := "gorage-backup-" + time.Now().Format("2006-01-02") + ".db"
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, tmp)
}

// ─── Recherche globale ───────────────────────────────────────────────────────

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	results := searchGlobal(q)
	renderPage(w, "search", PageData{
		Title:      "Recherche",
		ActivePage: "",
		Session:    getSession(r),
		Data:       results,
	})
}

// ─── Compte ──────────────────────────────────────────────────────────────────

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	ancien := r.FormValue("ancien_mdp")
	nouveau := strings.TrimSpace(r.FormValue("nouveau_mdp"))
	confirm := r.FormValue("confirm_mdp")

	redirect := func(err string) {
		http.Redirect(w, r, "/settings?err="+err+"#compte", http.StatusFound)
	}

	if ancien == "" || nouveau == "" || confirm == "" {
		redirect("Tous+les+champs+sont+obligatoires")
		return
	}
	if nouveau != confirm {
		redirect("Les+mots+de+passe+ne+correspondent+pas")
		return
	}
	if len(nouveau) < 6 {
		redirect("Le+mot+de+passe+doit+faire+au+moins+6+caractères")
		return
	}

	_, currentHash, _, err := getUserByUsername(sess.Username)
	if err != nil || !checkPassword(ancien, currentHash) {
		redirect("Mot+de+passe+actuel+incorrect")
		return
	}

	newHash, err := hashPassword(nouveau)
	if err != nil {
		redirect("Erreur+interne")
		return
	}
	if err := updatePassword(sess.UserID, newHash); err != nil {
		redirect("Erreur+lors+de+la+mise+à+jour")
		return
	}
	http.Redirect(w, r, "/settings?ok=Mot+de+passe+modifié#compte", http.StatusFound)
}

// ─── Paramètres du garage ─────────────────────────────────────────────────────

type SettingsPageData struct {
	S     map[string]string
	Users []UserInfo
}

func settingsGetHandler(w http.ResponseWriter, r *http.Request) {
	users, _ := getUsersList()
	renderPage(w, "settings", PageData{
		Title:      "Paramètres",
		ActivePage: "settings",
		Session:    getSession(r),
		Data:       SettingsPageData{S: getSettings(), Users: users},
		Success:    r.URL.Query().Get("ok"),
		Error:      r.URL.Query().Get("err"),
	})
}

func userCreateHandler(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	redirect := func(msg string) {
		http.Redirect(w, r, "/settings?err="+msg+"#utilisateurs", http.StatusFound)
	}

	if username == "" || password == "" {
		redirect("Nom+d'utilisateur+et+mot+de+passe+obligatoires")
		return
	}
	if len(password) < 6 {
		redirect("Mot+de+passe+:+6+caractères+minimum")
		return
	}
	if role != "admin" && role != "mecanicien" {
		role = "mecanicien"
	}

	hash, err := hashPassword(password)
	if err != nil {
		redirect("Erreur+interne")
		return
	}
	if err := createUserWithRole(username, hash, role); err != nil {
		redirect("Nom+d'utilisateur+déjà+utilisé")
		return
	}
	http.Redirect(w, r, "/settings?ok=Utilisateur+créé#utilisateurs", http.StatusFound)
}

func userDeleteHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)

	if id == sess.UserID {
		http.Redirect(w, r, "/settings?err=Impossible+de+supprimer+votre+propre+compte#utilisateurs", http.StatusFound)
		return
	}
	if err := deleteUserByID(id); err != nil {
		log.Println("deleteUser:", err)
		http.Redirect(w, r, "/settings?err=Impossible+de+supprimer+cet+utilisateur#utilisateurs", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/settings?ok=Utilisateur+supprimé#utilisateurs", http.StatusFound)
}

func settingsPostHandler(w http.ResponseWriter, r *http.Request) {
	// Collecte tous les champs connus — seuls ceux soumis seront non-vides
	data := map[string]string{}
	fields := []string{
		"nom_garage", "adresse_garage", "telephone_garage", "ville_garage",
		"nif", "nis", "rc", "ai",
		"tva_taux_defaut", "mentions_legales", "delai_validite_devis",
		"heure_ouverture", "heure_fermeture",
		"smtp_host", "smtp_port", "smtp_user", "smtp_password", "smtp_from_name", "smtp_from_email",
		"s3_endpoint", "s3_bucket", "s3_access_key", "s3_secret_key", "s3_region",
	}
	for _, f := range fields {
		if v := r.FormValue(f); v != "" || f == "tva_taux_defaut" {
			data[f] = strings.TrimSpace(v)
		}
	}

	// Détecter l'onglet actif depuis le Referer pour la redirection
	tab := "garage"
	if data["tva_taux_defaut"] != "" && data["nom_garage"] == "" {
		tab = "facturation"
	} else if data["heure_ouverture"] != "" && data["nom_garage"] == "" {
		tab = "atelier"
	}

	if err := updateSettings(data); err != nil {
		http.Redirect(w, r, "/settings?ok=&err=Erreur+sauvegarde#"+tab, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/settings?ok=Paramètres+enregistrés#"+tab, http.StatusFound)
}
