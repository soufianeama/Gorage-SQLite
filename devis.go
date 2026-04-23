package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════
// MODÈLES
// ═══════════════════════════════════════════════════════════

type DevisLigne struct {
	ID          int64
	DevisID     int64
	Ordre       int
	Type        string  // main_oeuvre | piece | fourniture | autre
	Designation string
	Quantite    float64
	PrixUnitHT  float64
	TVATaux     float64
	MontantHT   float64
	MontantTVA  float64
	MontantTTC  float64
}

type Devis struct {
	ID              int64
	Numero          string
	ClientID        int64
	ClientNom       string
	ClientTel       string
	ClientEmail     string
	ClientAdresseDB string
	VehiculeID      int64
	Immatriculation string
	VehiculeLabel   string // Marque + Modèle
	DateCreation    string
	DateValidite    string
	Description     string
	Statut          string // brouillon | envoye | accepte | refuse | facture
	MontantHT       float64
	MontantTVA      float64
	MontantTTC      float64
	ModeReglement   string
	Notes           string
	CreatedAt       string
	Lignes          []DevisLigne
}

// ═══════════════════════════════════════════════════════════
// NUMÉROTATION SÉQUENTIELLE
// ═══════════════════════════════════════════════════════════

func genDevisNumero() string {
	y := time.Now().Year()
	var n int
	db.QueryRow(
		`SELECT COALESCE(MAX(CAST(SUBSTR(numero,10) AS INTEGER)),0)+1
		 FROM devis WHERE numero LIKE ?`,
		fmt.Sprintf("DEV-%d-%%", y),
	).Scan(&n)
	return fmt.Sprintf("DEV-%d-%04d", y, n)
}

// ═══════════════════════════════════════════════════════════
// CRUD
// ═══════════════════════════════════════════════════════════

func createDevis(dv *Devis) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	dv.Numero = genDevisNumero()

	res, err := tx.Exec(`
		INSERT INTO devis
		  (numero,client_id,vehicule_id,date_creation,date_validite,
		   description,montant_ht,montant_tva,montant_ttc,mode_reglement,notes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		dv.Numero, dv.ClientID, dv.VehiculeID,
		dv.DateCreation, dv.DateValidite, dv.Description,
		dv.MontantHT, dv.MontantTVA, dv.MontantTTC,
		dv.ModeReglement, dv.Notes,
	)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return 0, err
	}
	id, _ := res.LastInsertId()

	for i, l := range dv.Lignes {
		_, err = tx.Exec(`
			INSERT INTO devis_lignes
			  (devis_id,ordre,type,designation,quantite,prix_unit_ht,tva_taux,
			   montant_ht,montant_tva,montant_ttc)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, i, l.Type, l.Designation, l.Quantite, l.PrixUnitHT, l.TVATaux,
			l.MontantHT, l.MontantTVA, l.MontantTTC,
		)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return 0, err
		}
	}
	return id, tx.Commit()
}

func getDevisList() ([]Devis, error) {
	rows, err := db.Query(`
		SELECT d.id, d.numero, d.client_id, c.nom||' '||c.prenom,
		       d.vehicule_id, v.immatriculation, v.marque||' '||v.modele,
		       d.date_creation, COALESCE(d.date_validite,''),
		       d.statut, d.montant_ht, d.montant_tva, d.montant_ttc,
		       COALESCE(d.mode_reglement,'especes'), d.created_at
		FROM devis d
		JOIN clients   c ON c.id = d.client_id
		JOIN vehicules v ON v.id = d.vehicule_id
		ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Devis
	for rows.Next() {
		var dv Devis
		rows.Scan(&dv.ID, &dv.Numero, &dv.ClientID, &dv.ClientNom,
			&dv.VehiculeID, &dv.Immatriculation, &dv.VehiculeLabel,
			&dv.DateCreation, &dv.DateValidite,
			&dv.Statut, &dv.MontantHT, &dv.MontantTVA, &dv.MontantTTC,
			&dv.ModeReglement, &dv.CreatedAt)
		list = append(list, dv)
	}
	return list, nil
}

func getDevisByID(id int64) (Devis, error) {
	var dv Devis
	err := db.QueryRow(`
		SELECT d.id, d.numero, d.client_id,
		       c.nom||' '||c.prenom, COALESCE(c.telephone,''), COALESCE(c.email,''), COALESCE(c.adresse,''),
		       d.vehicule_id, v.immatriculation, v.marque||' '||v.modele,
		       d.date_creation, COALESCE(d.date_validite,''), COALESCE(d.description,''),
		       d.statut, d.montant_ht, d.montant_tva, d.montant_ttc,
		       COALESCE(d.mode_reglement,'especes'), COALESCE(d.notes,''), d.created_at
		FROM devis d
		JOIN clients   c ON c.id = d.client_id
		JOIN vehicules v ON v.id = d.vehicule_id
		WHERE d.id = ?`, id,
	).Scan(&dv.ID, &dv.Numero, &dv.ClientID,
		&dv.ClientNom, &dv.ClientTel, &dv.ClientEmail, &dv.ClientAdresseDB,
		&dv.VehiculeID, &dv.Immatriculation, &dv.VehiculeLabel,
		&dv.DateCreation, &dv.DateValidite, &dv.Description,
		&dv.Statut, &dv.MontantHT, &dv.MontantTVA, &dv.MontantTTC,
		&dv.ModeReglement, &dv.Notes, &dv.CreatedAt)
	if err != nil {
		return dv, err
	}

	rows, _ := db.Query(`
		SELECT id,devis_id,ordre,type,designation,quantite,prix_unit_ht,
		       tva_taux,montant_ht,montant_tva,montant_ttc
		FROM devis_lignes WHERE devis_id=? ORDER BY ordre`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var l DevisLigne
			rows.Scan(&l.ID, &l.DevisID, &l.Ordre, &l.Type, &l.Designation,
				&l.Quantite, &l.PrixUnitHT, &l.TVATaux,
				&l.MontantHT, &l.MontantTVA, &l.MontantTTC)
			dv.Lignes = append(dv.Lignes, l)
		}
	}
	return dv, nil
}

func updateDevisStatut(id int64, statut string) error {
	_, err := db.Exec("UPDATE devis SET statut=? WHERE id=?", statut, id)
	return err
}

// Seuls les brouillons sont supprimables (anti-fraude)
func deleteDevis(id int64) error {
	_, err := db.Exec("DELETE FROM devis WHERE id=? AND statut='brouillon'", id)
	return err
}

// ═══════════════════════════════════════════════════════════
// HANDLERS
// ═══════════════════════════════════════════════════════════

func devisListHandler(w http.ResponseWriter, r *http.Request) {
	list, _ := getDevisList()
	renderPage(w, "devis_list", PageData{
		Title: "Devis & Ordres de Réparation", ActivePage: "devis",
		Session: getSession(r), Data: list,
		Success: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
	})
}

func devisNewHandler(w http.ResponseWriter, r *http.Request) {
	clients, _ := getClients()
	vehicules, _ := getVehicules()
	gs := getSettings()

	delai, _ := strconv.Atoi(gs["delai_validite_devis"])
	if delai <= 0 {
		delai = 30
	}
	dateValidite := time.Now().AddDate(0, 0, delai).Format("2006-01-02")

	type FD struct {
		Clients      []Client
		Vehicules    []Vehicule
		Today        string
		DateValidite string
		TVADefaut    string
	}
	renderPage(w, "devis_form", PageData{
		Title: "Nouveau devis", ActivePage: "devis",
		Session: getSession(r),
		Data:    FD{clients, vehicules, time.Now().Format("2006-01-02"), dateValidite, gs["tva_taux_defaut"]},
	})
}

func devisCreateHandler(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	vehiculeID, _ := strconv.ParseInt(r.FormValue("vehicule_id"), 10, 64)

	if clientID == 0 || vehiculeID == 0 {
		http.Redirect(w, r, "/devis/new?err=Client+et+véhicule+obligatoires", http.StatusFound)
		return
	}

	lignes := parseDevisLignes(r)
	if len(lignes) == 0 {
		http.Redirect(w, r, "/devis/new?err=Au+moins+une+ligne+est+requise", http.StatusFound)
		return
	}

	var ht, tva, ttc float64
	for _, l := range lignes {
		ht += l.MontantHT
		tva += l.MontantTVA
		ttc += l.MontantTTC
	}

	dv := &Devis{
		ClientID:      clientID,
		VehiculeID:    vehiculeID,
		DateCreation:  r.FormValue("date_creation"),
		DateValidite:  r.FormValue("date_validite"),
		Description:   r.FormValue("description"),
		MontantHT:     ht,
		MontantTVA:    tva,
		MontantTTC:    ttc,
		ModeReglement: r.FormValue("mode_reglement"),
		Notes:         r.FormValue("notes"),
		Lignes:        lignes,
	}

	id, err := createDevis(dv)
	if err != nil {
		log.Println("createDevis:", err)
		http.Redirect(w, r, "/devis/new?err=Erreur+lors+de+la+création+du+devis", http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&ok=Devis+%s+créé", id, dv.Numero), http.StatusFound)
}

func devisViewHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	dv, err := getDevisByID(id)
	if err != nil {
		http.Redirect(w, r, "/devis?err=Devis+introuvable", http.StatusFound)
		return
	}
	renderPage(w, "devis_detail", PageData{
		Title: dv.Numero, ActivePage: "devis",
		Session: getSession(r), Data: dv,
		Success: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
	})
}

func devisStatutHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	statut := r.FormValue("statut")
	allowed := map[string]bool{
		"envoye": true, "accepte": true, "refuse": true, "brouillon": true,
	}
	if !allowed[statut] {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Statut+invalide", id), http.StatusFound)
		return
	}
	updateDevisStatut(id, statut) //nolint:errcheck

	// Création automatique d'une intervention quand le devis est accepté
	if statut == "accepte" {
		dv, err := getDevisByID(id)
		if err == nil {
			desc := dv.Description
			if desc == "" {
				desc = "Travaux selon devis " + dv.Numero
			}
			createIntervention(dv.VehiculeID, time.Now().Format("2006-01-02"), desc, 0, 0) //nolint:errcheck
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&ok=Statut+mis+à+jour", id), http.StatusFound)
}

func devisDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err := deleteDevis(id); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Seuls+les+brouillons+sont+supprimables", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/devis?ok=Devis+supprimé", http.StatusFound)
}

// devisFacturerHandler : convertit un devis accepté en facture officielle
func devisFacturerHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	dv, err := getDevisByID(id)
	if err != nil {
		http.Redirect(w, r, "/devis?err=Devis+introuvable", http.StatusFound)
		return
	}
	if dv.Statut != "accepte" {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Le+devis+doit+être+à+l'état+Accepté", id), http.StatusFound)
		return
	}

	gs := getSettings()

	f := &FactureOfficielle{
		DevisID:         dv.ID,
		ClientID:        dv.ClientID,
		VehiculeID:      dv.VehiculeID,
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
		ClientNom:       dv.ClientNom,
		ClientAdresse:   dv.ClientAdresseDB,
		ClientTelephone: dv.ClientTel,
		ClientNIF:       r.FormValue("client_nif"),
		VehiculeImmat:   dv.Immatriculation,
		MontantHT:       dv.MontantHT,
		TVATaux:         parseFloatDef(gs["tva_taux_defaut"], 19),
		MontantTVA:      dv.MontantTVA,
		MontantTTC:      dv.MontantTTC,
		ModeReglement:   r.FormValue("mode_reglement"),
	}
	// Recopie des lignes (snapshot immuable)
	for _, l := range dv.Lignes {
		f.Lignes = append(f.Lignes, FactureLigne{
			Type: l.Type, Designation: l.Designation,
			Quantite:   l.Quantite,
			PrixUnitHT: l.PrixUnitHT, TVATaux: l.TVATaux,
			MontantHT: l.MontantHT, MontantTVA: l.MontantTVA, MontantTTC: l.MontantTTC,
		})
	}

	fid, err := createFactureOfficielle(f)
	if err != nil {
		log.Println("devisFacturer:", err)
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Erreur+lors+de+la+facturation", id), http.StatusFound)
		return
	}
	updateDevisStatut(id, "facture") //nolint:errcheck
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Facture+%s+émise", fid, f.Numero), http.StatusFound)
}

// ─── Helpers formulaire ───────────────────────────────────────────────────────

func parseDevisLignes(r *http.Request) []DevisLigne {
	types        := r.Form["ligne_type[]"]
	designations := r.Form["ligne_designation[]"]
	qtes         := r.Form["ligne_qte[]"]
	prix         := r.Form["ligne_prix[]"]
	tvas         := r.Form["ligne_tva[]"]

	var lignes []DevisLigne
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
		lignes = append(lignes, DevisLigne{
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

func safeStrGet(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "0"
}

// ligneTypeLabel retourne le libellé d'un type de ligne (utilisé dans les templates).
func ligneTypeLabel(t string) string {
	switch t {
	case "main_oeuvre":
		return "Main d'œuvre"
	case "piece":
		return "Pièce détachée"
	case "fourniture":
		return "Fourniture"
	default:
		return "Autre"
	}
}
