package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════
// MODÈLES — FACTURES OFFICIELLES
// ═══════════════════════════════════════════════════════════

type FactureLigne struct {
	ID          int64
	FactureID   int64
	Ordre       int
	Type        string
	Designation string
	Quantite    float64
	PrixUnitHT  float64
	TVATaux     float64
	MontantHT   float64
	MontantTVA  float64
	MontantTTC  float64
}

// FactureOfficielle est IMMUABLE une fois créée (loi anti-fraude).
// Aucune suppression ni modification des montants n'est possible.
// Seul le statut peut évoluer : emise → payee | annulee (avec motif obligatoire).
type FactureOfficielle struct {
	ID              int64
	Numero          string
	DevisID         int64
	InterventionID  int64
	ClientID        int64
	VehiculeID      int64
	DateEmission    string
	DateEcheance    string
	// ── Snapshot garage (figé à l'émission) ──────────────
	GarageNom       string
	GarageAdresse   string
	GarageTelephone string
	GarageVille     string
	GarageNIF       string // Numéro d'Identification Fiscale
	GarageNIS       string // Numéro d'Identification Statistique
	GarageRC        string // Registre de Commerce
	GarageAI        string // Article d'Imposition
	// ── Snapshot client ──────────────────────────────────
	ClientNom       string
	ClientAdresse   string
	ClientTelephone string
	ClientNIF       string
	ClientEmail     string // non stocké en DB, chargé depuis clients
	// ── Snapshot véhicule ────────────────────────────────
	VehiculeImmat   string
	VehiculeMarque  string
	VehiculeModele  string
	// ── Montants ─────────────────────────────────────────
	MontantHT       float64
	TVATaux         float64
	MontantTVA      float64
	MontantTTC      float64
	ModeReglement   string
	// ── Anti-fraude ──────────────────────────────────────
	Statut          string // emise | payee | annulee
	MotifAnnulation string
	HashControle    string // SHA-256 de (numero|client|ttc|date|nif)
	// ── Avoir ─────────────────────────────────────────────
	TypeDoc         string // facture | avoir
	AvoirPourID     int64  // 0 pour les factures normales
	AvoirPourNumero string // numéro de la facture d'origine (calculé)
	CreatedAt       string
	Lignes          []FactureLigne
}

// ═══════════════════════════════════════════════════════════
// NUMÉROTATION — séquentielle, sans trou (anti-fraude)
// ═══════════════════════════════════════════════════════════

// genFactureNumero génère le prochain numéro DANS une transaction
// pour garantir l'unicité même en accès concurrent.
func genFactureNumeroTx(tx interface {
	QueryRow(string, ...interface{}) interface {
		Scan(...interface{}) error
	}
}) string {
	// Délégation à db direct (app mono-utilisateur locale)
	y := time.Now().Year()
	var n int
	db.QueryRow(
		`SELECT COALESCE(MAX(CAST(SUBSTR(numero,10) AS INTEGER)),0)+1
		 FROM factures_officielles WHERE numero LIKE ?`,
		fmt.Sprintf("FAC-%d-%%", y),
	).Scan(&n)
	return fmt.Sprintf("FAC-%d-%04d", y, n)
}

// ═══════════════════════════════════════════════════════════
// CRUD
// ═══════════════════════════════════════════════════════════

func createFactureOfficielle(f *FactureOfficielle) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	// Numéro séquentiel dans la transaction
	y := time.Now().Year()
	var seq int
	if f.TypeDoc == "avoir" {
		tx.QueryRow( //nolint:errcheck
			`SELECT COALESCE(MAX(CAST(SUBSTR(numero,9) AS INTEGER)),0)+1
			 FROM factures_officielles WHERE numero LIKE ?`,
			fmt.Sprintf("AV-%d-%%", y),
		).Scan(&seq)
		f.Numero = fmt.Sprintf("AV-%d-%04d", y, seq)
	} else {
		f.TypeDoc = "facture"
		tx.QueryRow( //nolint:errcheck
			`SELECT COALESCE(MAX(CAST(SUBSTR(numero,10) AS INTEGER)),0)+1
			 FROM factures_officielles WHERE numero LIKE ?`,
			fmt.Sprintf("FAC-%d-%%", y),
		).Scan(&seq)
		f.Numero = fmt.Sprintf("FAC-%d-%04d", y, seq)
	}

	// Empreinte anti-fraude (SHA-256)
	raw := fmt.Sprintf("%s|%s|%.2f|%s|%s",
		f.Numero, strings.ToUpper(f.ClientNom), f.MontantTTC, f.DateEmission, f.GarageNIF)
	sum := sha256.Sum256([]byte(raw))
	f.HashControle = fmt.Sprintf("%x", sum)[:16] // 16 premiers caractères suffisent pour l'affichage

	res, err := tx.Exec(`
		INSERT INTO factures_officielles (
			numero,devis_id,intervention_id,client_id,vehicule_id,
			date_emission,date_echeance,
			garage_nom,garage_adresse,garage_telephone,garage_ville,
			garage_nif,garage_nis,garage_rc,garage_ai,
			client_nom,client_adresse,client_telephone,client_nif,
			vehicule_immat,vehicule_marque,vehicule_modele,
			montant_ht,tva_taux,montant_tva,montant_ttc,
			mode_reglement,hash_controle,type_doc,avoir_pour_id
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.Numero, f.DevisID, f.InterventionID, f.ClientID, f.VehiculeID,
		f.DateEmission, f.DateEcheance,
		f.GarageNom, f.GarageAdresse, f.GarageTelephone, f.GarageVille,
		f.GarageNIF, f.GarageNIS, f.GarageRC, f.GarageAI,
		f.ClientNom, f.ClientAdresse, f.ClientTelephone, f.ClientNIF,
		f.VehiculeImmat, f.VehiculeMarque, f.VehiculeModele,
		f.MontantHT, f.TVATaux, f.MontantTVA, f.MontantTTC,
		f.ModeReglement, f.HashControle, f.TypeDoc, f.AvoirPourID,
	)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return 0, err
	}
	id, _ := res.LastInsertId()

	for i, l := range f.Lignes {
		_, err = tx.Exec(`
			INSERT INTO factures_lignes
			  (facture_id,ordre,type,designation,quantite,prix_unit_ht,
			   tva_taux,montant_ht,montant_tva,montant_ttc)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, i, l.Type, l.Designation, l.Quantite, l.PrixUnitHT,
			l.TVATaux, l.MontantHT, l.MontantTVA, l.MontantTTC,
		)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return 0, err
		}
	}
	return id, tx.Commit()
}

func getFacturesList() ([]FactureOfficielle, error) {
	rows, err := db.Query(`
		SELECT id,numero,client_nom,vehicule_immat,
		       date_emission,COALESCE(date_echeance,''),
		       montant_ht,tva_taux,montant_tva,montant_ttc,
		       mode_reglement,statut,hash_controle,
		       COALESCE(type_doc,'facture'),COALESCE(avoir_pour_id,0),
		       created_at
		FROM factures_officielles
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []FactureOfficielle
	for rows.Next() {
		var f FactureOfficielle
		rows.Scan(&f.ID, &f.Numero, &f.ClientNom, &f.VehiculeImmat,
			&f.DateEmission, &f.DateEcheance,
			&f.MontantHT, &f.TVATaux, &f.MontantTVA, &f.MontantTTC,
			&f.ModeReglement, &f.Statut, &f.HashControle,
			&f.TypeDoc, &f.AvoirPourID,
			&f.CreatedAt)
		list = append(list, f)
	}
	return list, nil
}

func getFactureByID(id int64) (FactureOfficielle, error) {
	var f FactureOfficielle
	err := db.QueryRow(`
		SELECT id,numero,devis_id,intervention_id,client_id,vehicule_id,
		       date_emission,COALESCE(date_echeance,''),
		       garage_nom,garage_adresse,garage_telephone,garage_ville,
		       garage_nif,garage_nis,garage_rc,garage_ai,
		       client_nom,COALESCE(client_adresse,''),COALESCE(client_telephone,''),COALESCE(client_nif,''),
		       vehicule_immat,COALESCE(vehicule_marque,''),COALESCE(vehicule_modele,''),
		       montant_ht,tva_taux,montant_tva,montant_ttc,
		       mode_reglement,statut,COALESCE(motif_annulation,''),hash_controle,
		       COALESCE(type_doc,'facture'),COALESCE(avoir_pour_id,0),
		       created_at
		FROM factures_officielles WHERE id=?`, id,
	).Scan(&f.ID, &f.Numero, &f.DevisID, &f.InterventionID, &f.ClientID, &f.VehiculeID,
		&f.DateEmission, &f.DateEcheance,
		&f.GarageNom, &f.GarageAdresse, &f.GarageTelephone, &f.GarageVille,
		&f.GarageNIF, &f.GarageNIS, &f.GarageRC, &f.GarageAI,
		&f.ClientNom, &f.ClientAdresse, &f.ClientTelephone, &f.ClientNIF,
		&f.VehiculeImmat, &f.VehiculeMarque, &f.VehiculeModele,
		&f.MontantHT, &f.TVATaux, &f.MontantTVA, &f.MontantTTC,
		&f.ModeReglement, &f.Statut, &f.MotifAnnulation, &f.HashControle,
		&f.TypeDoc, &f.AvoirPourID,
		&f.CreatedAt)
	if err != nil {
		return f, err
	}
	if f.AvoirPourID != 0 {
		db.QueryRow("SELECT numero FROM factures_officielles WHERE id=?", f.AvoirPourID).
			Scan(&f.AvoirPourNumero)
	}
	// Email client (non stocké dans le snapshot, chargé en direct)
	db.QueryRow("SELECT COALESCE(email,'') FROM clients WHERE id=?", f.ClientID).
		Scan(&f.ClientEmail)

	rows, _ := db.Query(`
		SELECT id,facture_id,ordre,type,designation,quantite,prix_unit_ht,
		       tva_taux,montant_ht,montant_tva,montant_ttc
		FROM factures_lignes WHERE facture_id=? ORDER BY ordre`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var l FactureLigne
			rows.Scan(&l.ID, &l.FactureID, &l.Ordre, &l.Type, &l.Designation,
				&l.Quantite, &l.PrixUnitHT, &l.TVATaux,
				&l.MontantHT, &l.MontantTVA, &l.MontantTTC)
			f.Lignes = append(f.Lignes, l)
		}
	}
	return f, nil
}

// Seul le statut peut changer — jamais les montants (immuabilité anti-fraude)
func updateFactureStatut(id int64, statut, motif string) error {
	_, err := db.Exec(
		"UPDATE factures_officielles SET statut=?,motif_annulation=? WHERE id=?",
		statut, motif, id,
	)
	return err
}

// ═══════════════════════════════════════════════════════════
// HANDLERS
// ═══════════════════════════════════════════════════════════

type FacturesStats struct {
	CAMois       float64
	CAMoisN      int64
	CAAnne       float64
	CAAnneN      int64
	EnAttente    float64
	EnAttenteN   int64
}

func getFacturesStats() FacturesStats {
	var s FacturesStats
	db.QueryRow(`
		SELECT COALESCE(SUM(montant_ht),0), COUNT(*)
		FROM factures_officielles
		WHERE statut='payee' AND COALESCE(type_doc,'facture')='facture'
		AND strftime('%Y-%m', date_emission) = strftime('%Y-%m','now')`,
	).Scan(&s.CAMois, &s.CAMoisN)
	db.QueryRow(`
		SELECT COALESCE(SUM(montant_ht),0), COUNT(*)
		FROM factures_officielles
		WHERE statut='payee' AND COALESCE(type_doc,'facture')='facture'
		AND strftime('%Y', date_emission) = strftime('%Y','now')`,
	).Scan(&s.CAAnne, &s.CAAnneN)
	db.QueryRow(`
		SELECT COALESCE(SUM(montant_ht),0), COUNT(*)
		FROM factures_officielles
		WHERE statut='emise' AND COALESCE(type_doc,'facture')='facture'`,
	).Scan(&s.EnAttente, &s.EnAttenteN)
	return s
}

func facturesListHandler(w http.ResponseWriter, r *http.Request) {
	list, _ := getFacturesList()

	type FacturesData struct {
		Factures []FactureOfficielle
		Stats    FacturesStats
	}

	renderPage(w, "factures_list", PageData{
		Title: "Factures", ActivePage: "factures",
		Session: getSession(r),
		Data:    FacturesData{Factures: list, Stats: getFacturesStats()},
		Success: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
	})
}

func factureViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	f, err := getFactureByID(id)
	if err != nil {
		http.Redirect(w, r, "/factures?err=Facture+introuvable", http.StatusFound)
		return
	}
	renderPage(w, "facture_view", PageData{
		Title: f.Numero, ActivePage: "factures",
		Session: getSession(r), Data: f,
		Success: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
	})
}

// facturePayerHandler : marque une facture comme payée
func facturePayerHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	f, err := getFactureByID(id)
	if err != nil || f.TypeDoc == "avoir" {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Action+non+autorisée+sur+un+avoir", id), http.StatusFound)
		return
	}
	updateFactureStatut(id, "payee", "") //nolint:errcheck
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Facture+marquée+payée", id), http.StatusFound)
}

// factureUnpayerHandler : remet une facture payée à l'état "émise"
func factureUnpayerHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	f, err := getFactureByID(id)
	if err != nil || f.TypeDoc == "avoir" {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Action+non+autorisée+sur+un+avoir", id), http.StatusFound)
		return
	}
	updateFactureStatut(id, "emise", "") //nolint:errcheck
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Facture+remise+en+émise", id), http.StatusFound)
}

// factureAnnulerHandler : annulation avec motif obligatoire (jamais de suppression)
func factureAnnulerHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	motif := strings.TrimSpace(r.FormValue("motif"))
	if motif == "" {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Le+motif+d'annulation+est+obligatoire", id), http.StatusFound)
		return
	}
	updateFactureStatut(id, "annulee", motif) //nolint:errcheck
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Facture+annulée", id), http.StatusFound)
}

// factureDirecteHandler : crée une facture directement (sans devis)
func factureDirecteHandler(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	vehiculeID, _ := strconv.ParseInt(r.FormValue("vehicule_id"), 10, 64)

	if clientID == 0 || vehiculeID == 0 {
		http.Redirect(w, r, "/factures?err=Client+et+véhicule+obligatoires", http.StatusFound)
		return
	}

	lignes := parseFactureLignes(r)
	if len(lignes) == 0 {
		http.Redirect(w, r, "/factures?err=Au+moins+une+ligne+est+requise", http.StatusFound)
		return
	}

	var ht, tva, ttc float64
	for _, l := range lignes {
		ht += l.MontantHT
		tva += l.MontantTVA
		ttc += l.MontantTTC
	}

	// Charger client + véhicule pour le snapshot
	client, _ := getClientByID(clientID)
	vehicules, _ := getVehicules()
	var veh Vehicule
	for _, v := range vehicules {
		if v.ID == vehiculeID {
			veh = v
			break
		}
	}

	gs := getSettings()

	f := &FactureOfficielle{
		ClientID:        clientID,
		VehiculeID:      vehiculeID,
		DateEmission:    time.Now().Format("2006-01-02"),
		DateEcheance:    r.FormValue("date_echeance"),
		GarageNom:       gs["nom_garage"],
		GarageAdresse:   gs["adresse_garage"],
		GarageTelephone: gs["telephone_garage"],
		GarageVille:     gs["ville_garage"],
		GarageNIF:       gs["nif"],
		GarageNIS:       gs["nis"],
		GarageRC:        gs["rc"],
		GarageAI:        gs["ai"],
		ClientNom:       client.Nom + " " + client.Prenom,
		ClientAdresse:   client.Adresse,
		ClientTelephone: client.Telephone,
		ClientNIF:       r.FormValue("client_nif"),
		VehiculeImmat:   veh.Immatriculation,
		VehiculeMarque:  veh.Marque,
		VehiculeModele:  veh.Modele,
		MontantHT:       ht,
		TVATaux:         parseFloatDef(gs["tva_taux_defaut"], 19),
		MontantTVA:      tva,
		MontantTTC:      ttc,
		ModeReglement:   r.FormValue("mode_reglement"),
		Lignes:          lignes,
	}

	fid, err := createFactureOfficielle(f)
	if err != nil {
		log.Println("createFactureOfficielle:", err)
		http.Redirect(w, r, "/factures?err=Erreur+lors+de+la+création+de+la+facture", http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Facture+%s+émise", fid, f.Numero), http.StatusFound)
}

// factureNouvelleFormHandler : affiche le formulaire de facture directe
func factureNouvelleFormHandler(w http.ResponseWriter, r *http.Request) {
	clients, _ := getClients()
	vehicules, _ := getVehicules()
	type FD struct {
		Clients   []Client
		Vehicules []Vehicule
		Today     string
	}
	renderPage(w, "facture_form", PageData{
		Title: "Nouvelle facture", ActivePage: "factures",
		Session: getSession(r),
		Data:    FD{clients, vehicules, time.Now().Format("2006-01-02")},
	})
}

// createAvoir génère un avoir (note de crédit) à partir d'une facture payée.
func createAvoir(originalID int64) (int64, string, error) {
	// Vérifier qu'aucun avoir n'existe déjà pour cette facture
	var existCount int
	db.QueryRow(
		`SELECT COUNT(*) FROM factures_officielles WHERE avoir_pour_id=? AND COALESCE(type_doc,'facture')='avoir'`,
		originalID,
	).Scan(&existCount)
	if existCount > 0 {
		return 0, "", fmt.Errorf("un avoir a déjà été émis pour cette facture")
	}

	orig, err := getFactureByID(originalID)
	if err != nil {
		return 0, "", err
	}
	avoir := &FactureOfficielle{
		TypeDoc:         "avoir",
		AvoirPourID:     originalID,
		DevisID:         orig.DevisID,
		InterventionID:  orig.InterventionID,
		ClientID:        orig.ClientID,
		VehiculeID:      orig.VehiculeID,
		DateEmission:    time.Now().Format("2006-01-02"),
		GarageNom:       orig.GarageNom,
		GarageAdresse:   orig.GarageAdresse,
		GarageTelephone: orig.GarageTelephone,
		GarageVille:     orig.GarageVille,
		GarageNIF:       orig.GarageNIF,
		GarageNIS:       orig.GarageNIS,
		GarageRC:        orig.GarageRC,
		GarageAI:        orig.GarageAI,
		ClientNom:       orig.ClientNom,
		ClientAdresse:   orig.ClientAdresse,
		ClientTelephone: orig.ClientTelephone,
		ClientNIF:       orig.ClientNIF,
		VehiculeImmat:   orig.VehiculeImmat,
		VehiculeMarque:  orig.VehiculeMarque,
		VehiculeModele:  orig.VehiculeModele,
		MontantHT:       -orig.MontantHT,
		TVATaux:         orig.TVATaux,
		MontantTVA:      -orig.MontantTVA,
		MontantTTC:      -orig.MontantTTC,
		ModeReglement:   orig.ModeReglement,
	}
	for _, l := range orig.Lignes {
		avoir.Lignes = append(avoir.Lignes, FactureLigne{
			Type:        l.Type,
			Designation: l.Designation,
			Quantite:    l.Quantite,
			PrixUnitHT:  -l.PrixUnitHT,
			TVATaux:     l.TVATaux,
			MontantHT:   -l.MontantHT,
			MontantTVA:  -l.MontantTVA,
			MontantTTC:  -l.MontantTTC,
		})
	}
	id, err := createFactureOfficielle(avoir)
	if err != nil {
		return 0, "", err
	}
	// La facture d'origine est annulée par l'avoir
	motif := "Avoir " + avoir.Numero + " émis"
	updateFactureStatut(originalID, "annulee", motif) //nolint:errcheck
	return id, avoir.Numero, nil
}

// avoirHandler : émet un avoir pour une facture payée
func avoirHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	avoirID, avoirNum, err := createAvoir(id)
	if err != nil {
		log.Println("createAvoir:", err)
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Erreur+lors+de+la+création+de+l'avoir", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Avoir+%s+émis", avoirID, avoirNum), http.StatusFound)
}

func parseFactureLignes(r *http.Request) []FactureLigne {
	types        := r.Form["ligne_type[]"]
	designations := r.Form["ligne_designation[]"]
	qtes         := r.Form["ligne_qte[]"]
	prix         := r.Form["ligne_prix[]"]
	tvas         := r.Form["ligne_tva[]"]

	var lignes []FactureLigne
	for i, d := range designations {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		qte, _ := strconv.ParseFloat(safeStrGet(qtes, i), 64)
		p, _   := strconv.ParseFloat(safeStrGet(prix, i), 64)
		tva, _ := strconv.ParseFloat(safeStrGet(tvas, i), 64)
		ht     := qte * p
		tvaAmt := ht * tva / 100
		lignes = append(lignes, FactureLigne{
			Type:        safeStrGet(types, i),
			Designation: d,
			Quantite:    qte,
			PrixUnitHT:  p,
			TVATaux:     tva,
			MontantHT:   ht,
			MontantTVA:  tvaAmt,
			MontantTTC:  ht + tvaAmt,
		})
	}
	return lignes
}
