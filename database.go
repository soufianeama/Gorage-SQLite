package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// ─── Modèles ────────────────────────────────────────────────────────────────

type Client struct {
	ID        int64
	Nom       string
	Prenom    string
	Telephone string
	Email     string
	Adresse   string
	CreatedAt string
}

type Vehicule struct {
	ID              int64
	ClientID        int64
	ClientNom       string
	Marque          string
	Modele          string
	Annee           int64
	Immatriculation string
	VIN             string
	Kilometrage     int64
	CreatedAt       string
}

type Intervention struct {
	ID               int64
	VehiculeID       int64
	Immatriculation  string
	ClientNom        string
	ClientTel        string
	ClientAdresse    string
	Marque           string
	Modele           string
	DateEntree       string
	DateSortie       string
	Description      string
	Diagnostic       string
	TravauxEffectues string
	PiecesUtilisees  string
	CoutMainOeuvre   float64
	CoutPieces       float64
	MontantTotal     float64
	Statut           string
	CreatedAt        string
}

type Stats struct {
	TotalClients         int64
	TotalVehicules       int64
	InterventionsActives int64
	CAMois               float64
	RDVAujourdhui        int64
	PiecesStockBas       int64
}

type Piece struct {
	ID            int64
	Reference     string
	Nom           string
	Categorie     string
	QuantiteStock int64
	SeuilAlerte   int64
	PrixAchat     float64
	PrixVente     float64
	Fournisseur   string
	Notes         string
	CreatedAt     string
	StockBas      bool // calculé : QuantiteStock <= SeuilAlerte
}

type RendezVous struct {
	ID              int64
	ClientID        int64
	VehiculeID      int64
	ClientNom       string
	ClientTel       string
	Immatriculation string
	Marque          string
	Modele          string
	DateRDV         string
	HeureRDV        string
	DureeMin        int
	Motif           string
	Notes           string
	Statut          string // planifie | confirme | annule | converti
	CreatedAt       string
}

// ─── Initialisation ─────────────────────────────────────────────────────────

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./gorage.db")
	if err != nil {
		log.Fatal("Erreur ouverture base de données :", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Erreur connexion base de données :", err)
	}

	db.Exec("PRAGMA foreign_keys = ON")
	db.Exec("PRAGMA journal_mode = WAL")

	createTables()
	migrateDB()
	log.Println("Base de données initialisée.")
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			cle    TEXT PRIMARY KEY,
			valeur TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS devis (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			numero         TEXT    NOT NULL UNIQUE,
			client_id      INTEGER NOT NULL,
			vehicule_id    INTEGER NOT NULL,
			date_creation  DATE    NOT NULL,
			date_validite  DATE    DEFAULT '',
			description    TEXT    DEFAULT '',
			statut         TEXT    NOT NULL DEFAULT 'brouillon',
			montant_ht     REAL    DEFAULT 0,
			montant_tva    REAL    DEFAULT 0,
			montant_ttc    REAL    DEFAULT 0,
			mode_reglement TEXT    DEFAULT 'especes',
			notes          TEXT    DEFAULT '',
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (client_id)   REFERENCES clients(id),
			FOREIGN KEY (vehicule_id) REFERENCES vehicules(id)
		)`,
		`CREATE TABLE IF NOT EXISTS devis_lignes (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			devis_id     INTEGER NOT NULL,
			ordre        INTEGER DEFAULT 0,
			type         TEXT    DEFAULT 'piece',
			designation  TEXT    NOT NULL,
			quantite     REAL    DEFAULT 1,
			prix_unit_ht REAL    DEFAULT 0,
			tva_taux     REAL    DEFAULT 19,
			montant_ht   REAL    DEFAULT 0,
			montant_tva  REAL    DEFAULT 0,
			montant_ttc  REAL    DEFAULT 0,
			FOREIGN KEY (devis_id) REFERENCES devis(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS factures_officielles (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			numero           TEXT NOT NULL UNIQUE,
			devis_id         INTEGER DEFAULT 0,
			intervention_id  INTEGER DEFAULT 0,
			client_id        INTEGER NOT NULL,
			vehicule_id      INTEGER NOT NULL,
			date_emission    DATE    NOT NULL,
			date_echeance    DATE    DEFAULT '',
			garage_nom       TEXT    NOT NULL,
			garage_adresse   TEXT    DEFAULT '',
			garage_telephone TEXT    DEFAULT '',
			garage_ville     TEXT    DEFAULT '',
			garage_nif       TEXT    DEFAULT '',
			garage_nis       TEXT    DEFAULT '',
			garage_rc        TEXT    DEFAULT '',
			garage_ai        TEXT    DEFAULT '',
			client_nom       TEXT    NOT NULL,
			client_adresse   TEXT    DEFAULT '',
			client_telephone TEXT    DEFAULT '',
			client_nif       TEXT    DEFAULT '',
			vehicule_immat   TEXT    NOT NULL,
			vehicule_marque  TEXT    DEFAULT '',
			vehicule_modele  TEXT    DEFAULT '',
			montant_ht       REAL    DEFAULT 0,
			tva_taux         REAL    DEFAULT 19,
			montant_tva      REAL    DEFAULT 0,
			montant_ttc      REAL    DEFAULT 0,
			mode_reglement   TEXT    DEFAULT 'especes',
			statut           TEXT    NOT NULL DEFAULT 'emise',
			motif_annulation TEXT    DEFAULT '',
			hash_controle    TEXT    DEFAULT '',
			type_doc         TEXT    DEFAULT 'facture',
			avoir_pour_id    INTEGER DEFAULT 0,
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS factures_lignes (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			facture_id   INTEGER NOT NULL,
			ordre        INTEGER DEFAULT 0,
			type         TEXT    DEFAULT 'piece',
			designation  TEXT    NOT NULL,
			quantite     REAL    DEFAULT 1,
			prix_unit_ht REAL    DEFAULT 0,
			tva_taux     REAL    DEFAULT 19,
			montant_ht   REAL    DEFAULT 0,
			montant_tva  REAL    DEFAULT 0,
			montant_ttc  REAL    DEFAULT 0,
			FOREIGN KEY (facture_id) REFERENCES factures_officielles(id)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL,
			role          TEXT    NOT NULL DEFAULT 'admin',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS clients (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			nom        TEXT NOT NULL,
			prenom     TEXT NOT NULL,
			telephone  TEXT DEFAULT '',
			email      TEXT DEFAULT '',
			adresse    TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS vehicules (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id        INTEGER NOT NULL,
			marque           TEXT    NOT NULL,
			modele           TEXT    NOT NULL,
			annee            INTEGER DEFAULT 0,
			immatriculation  TEXT    NOT NULL UNIQUE,
			vin              TEXT    DEFAULT '',
			kilometrage      INTEGER DEFAULT 0,
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS rendezvous (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id   INTEGER NOT NULL,
			vehicule_id INTEGER NOT NULL,
			date_rdv    TEXT    NOT NULL,
			heure_rdv   TEXT    NOT NULL DEFAULT '09:00',
			duree_min   INTEGER          DEFAULT 60,
			motif       TEXT    NOT NULL,
			notes       TEXT             DEFAULT '',
			statut      TEXT    NOT NULL DEFAULT 'planifie',
			created_at  DATETIME         DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (client_id)   REFERENCES clients(id),
			FOREIGN KEY (vehicule_id) REFERENCES vehicules(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS interventions (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			vehicule_id       INTEGER NOT NULL,
			date_entree       DATE    NOT NULL,
			date_sortie       DATE    DEFAULT '',
			description       TEXT    NOT NULL,
			diagnostic        TEXT    DEFAULT '',
			travaux_effectues TEXT    DEFAULT '',
			pieces_utilisees  TEXT    DEFAULT '',
			cout_main_oeuvre  REAL    DEFAULT 0,
			cout_pieces       REAL    DEFAULT 0,
			montant_total     REAL    DEFAULT 0,
			statut            TEXT    NOT NULL DEFAULT 'en_cours',
			created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (vehicule_id) REFERENCES vehicules(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS pieces (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			reference      TEXT    DEFAULT '',
			nom            TEXT    NOT NULL,
			categorie      TEXT    DEFAULT '',
			quantite_stock INTEGER DEFAULT 0,
			seuil_alerte   INTEGER DEFAULT 5,
			prix_achat     REAL    DEFAULT 0,
			prix_vente     REAL    DEFAULT 0,
			fournisseur    TEXT    DEFAULT '',
			notes          TEXT    DEFAULT '',
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Fatalf("Erreur création table : %v\n%s", err, q)
		}
	}
}

func migrateDB() {
	// Avoirs — ignored if columns already exist
	db.Exec("ALTER TABLE factures_officielles ADD COLUMN type_doc TEXT DEFAULT 'facture'")
	db.Exec("ALTER TABLE factures_officielles ADD COLUMN avoir_pour_id INTEGER DEFAULT 0")
}

// ─── Users ───────────────────────────────────────────────────────────────────

type UserInfo struct {
	ID        int64
	Username  string
	Role      string
	CreatedAt string
}

func userExists() bool {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n > 0
}

func createUser(username, hash string) error {
	_, err := db.Exec(
		"INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, hash,
	)
	return err
}

func getUserByUsername(username string) (id int64, hash, role string, err error) {
	err = db.QueryRow(
		"SELECT id, password_hash, role FROM users WHERE username = ?", username,
	).Scan(&id, &hash, &role)
	return
}

func getUsersList() ([]UserInfo, error) {
	rows, err := db.Query("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []UserInfo
	for rows.Next() {
		var u UserInfo
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
		list = append(list, u)
	}
	return list, nil
}

func createUserWithRole(username, passwordHash, role string) error {
	_, err := db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?,?,?)",
		username, passwordHash, role,
	)
	return err
}

func deleteUserByID(id int64) error {
	var role string
	db.QueryRow("SELECT role FROM users WHERE id=?", id).Scan(&role)
	if role == "admin" {
		var adminCount int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&adminCount)
		if adminCount <= 1 {
			return fmt.Errorf("impossible de supprimer le dernier administrateur")
		}
	}
	_, err := db.Exec("DELETE FROM users WHERE id=?", id)
	return err
}

func updatePassword(userID int64, newHash string) error {
	_, err := db.Exec("UPDATE users SET password_hash=? WHERE id=?", newHash, userID)
	return err
}

// ─── Clients ─────────────────────────────────────────────────────────────────

func getClients() ([]Client, error) {
	rows, err := db.Query(
		`SELECT id, nom, prenom, telephone, email, adresse, created_at
		 FROM clients ORDER BY nom, prenom`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Client
	for rows.Next() {
		var c Client
		rows.Scan(&c.ID, &c.Nom, &c.Prenom, &c.Telephone, &c.Email, &c.Adresse, &c.CreatedAt)
		list = append(list, c)
	}
	return list, nil
}

func getClientByID(id int64) (Client, error) {
	var c Client
	err := db.QueryRow(
		`SELECT id, nom, prenom, telephone, email, adresse, created_at
		 FROM clients WHERE id = ?`, id,
	).Scan(&c.ID, &c.Nom, &c.Prenom, &c.Telephone, &c.Email, &c.Adresse, &c.CreatedAt)
	return c, err
}

func createClient(nom, prenom, tel, email, adresse string) (int64, error) {
	r, err := db.Exec(
		"INSERT INTO clients (nom, prenom, telephone, email, adresse) VALUES (?, ?, ?, ?, ?)",
		nom, prenom, tel, email, adresse,
	)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func updateClient(id int64, nom, prenom, tel, email, adresse string) error {
	_, err := db.Exec(
		"UPDATE clients SET nom=?, prenom=?, telephone=?, email=?, adresse=? WHERE id=?",
		nom, prenom, tel, email, adresse, id,
	)
	return err
}

func deleteClient(id int64) error {
	_, err := db.Exec("DELETE FROM clients WHERE id = ?", id)
	return err
}

// ─── Véhicules ───────────────────────────────────────────────────────────────

func getVehicules() ([]Vehicule, error) {
	rows, err := db.Query(`
		SELECT v.id, v.client_id, c.nom||' '||c.prenom,
		       v.marque, v.modele, v.annee, v.immatriculation,
		       v.vin, v.kilometrage, v.created_at
		FROM vehicules v
		JOIN clients c ON c.id = v.client_id
		ORDER BY v.immatriculation`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Vehicule
	for rows.Next() {
		var v Vehicule
		rows.Scan(&v.ID, &v.ClientID, &v.ClientNom, &v.Marque, &v.Modele,
			&v.Annee, &v.Immatriculation, &v.VIN, &v.Kilometrage, &v.CreatedAt)
		list = append(list, v)
	}
	return list, nil
}

func createVehicule(clientID int64, marque, modele string, annee int64, immat, vin string, km int64) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO vehicules (client_id, marque, modele, annee, immatriculation, vin, kilometrage)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		clientID, marque, modele, annee, immat, vin, km,
	)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func getInterventionsByVehicule(vehiculeID int64) ([]Intervention, error) {
	rows, err := db.Query(`
		SELECT i.id, i.vehicule_id, v.immatriculation,
		       c.nom||' '||c.prenom, COALESCE(c.telephone,''), COALESCE(c.adresse,''),
		       v.marque, v.modele,
		       i.date_entree, COALESCE(i.date_sortie,''), i.description,
		       COALESCE(i.diagnostic,''), COALESCE(i.travaux_effectues,''),
		       COALESCE(i.pieces_utilisees,''),
		       i.cout_main_oeuvre, i.cout_pieces, i.montant_total,
		       i.statut, i.created_at
		FROM interventions i
		JOIN vehicules v ON v.id = i.vehicule_id
		JOIN clients   c ON c.id = v.client_id
		WHERE i.vehicule_id = ?
		ORDER BY i.date_entree DESC, i.id DESC`, vehiculeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Intervention
	for rows.Next() {
		var inv Intervention
		rows.Scan(&inv.ID, &inv.VehiculeID, &inv.Immatriculation,
			&inv.ClientNom, &inv.ClientTel, &inv.ClientAdresse,
			&inv.Marque, &inv.Modele,
			&inv.DateEntree, &inv.DateSortie, &inv.Description,
			&inv.Diagnostic, &inv.TravauxEffectues, &inv.PiecesUtilisees,
			&inv.CoutMainOeuvre, &inv.CoutPieces, &inv.MontantTotal,
			&inv.Statut, &inv.CreatedAt)
		list = append(list, inv)
	}
	return list, nil
}

func getDevisByVehicule(vehiculeID int64) ([]Devis, error) {
	rows, err := db.Query(`
		SELECT d.id, d.numero, d.client_id, c.nom||' '||c.prenom,
		       d.vehicule_id, v.immatriculation, v.marque||' '||v.modele,
		       d.date_creation, COALESCE(d.date_validite,''),
		       d.statut, d.montant_ht, d.montant_tva, d.montant_ttc,
		       COALESCE(d.mode_reglement,'especes'), d.created_at
		FROM devis d
		JOIN clients   c ON c.id = d.client_id
		JOIN vehicules v ON v.id = d.vehicule_id
		WHERE d.vehicule_id = ?
		ORDER BY d.created_at DESC`, vehiculeID)
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

func getRDVsByVehicule(vehiculeID int64) ([]RendezVous, error) {
	rows, err := db.Query(rdvSelectBase+
		` WHERE r.vehicule_id = ? ORDER BY r.date_rdv DESC, r.heure_rdv DESC`,
		vehiculeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RendezVous
	for rows.Next() {
		rdv, err := scanRDV(rows)
		if err == nil {
			list = append(list, rdv)
		}
	}
	return list, nil
}

func getVehiculeByID(id int64) (Vehicule, error) {
	var v Vehicule
	err := db.QueryRow(`
		SELECT v.id, v.client_id, c.nom||' '||c.prenom,
		       v.marque, v.modele, v.annee, v.immatriculation,
		       v.vin, v.kilometrage, v.created_at
		FROM vehicules v
		JOIN clients c ON c.id = v.client_id
		WHERE v.id = ?`, id,
	).Scan(&v.ID, &v.ClientID, &v.ClientNom, &v.Marque, &v.Modele,
		&v.Annee, &v.Immatriculation, &v.VIN, &v.Kilometrage, &v.CreatedAt)
	return v, err
}

func updateVehicule(id, clientID int64, marque, modele string, annee int64, immat, vin string, km int64) error {
	_, err := db.Exec(`
		UPDATE vehicules
		SET client_id=?, marque=?, modele=?, annee=?, immatriculation=?, vin=?, kilometrage=?
		WHERE id=?`,
		clientID, marque, modele, annee, immat, vin, km, id,
	)
	return err
}

func deleteVehicule(id int64) error {
	_, err := db.Exec("DELETE FROM vehicules WHERE id = ?", id)
	return err
}

// ─── Interventions ───────────────────────────────────────────────────────────

func getInterventions(statut string) ([]Intervention, error) {
	q := `
		SELECT i.id, i.vehicule_id, v.immatriculation,
		       c.nom||' '||c.prenom, COALESCE(c.telephone,''), COALESCE(c.adresse,''),
		       v.marque, v.modele,
		       i.date_entree, COALESCE(i.date_sortie,''), i.description,
		       COALESCE(i.diagnostic,''), COALESCE(i.travaux_effectues,''),
		       COALESCE(i.pieces_utilisees,''),
		       i.cout_main_oeuvre, i.cout_pieces, i.montant_total,
		       i.statut, i.created_at
		FROM interventions i
		JOIN vehicules v ON v.id = i.vehicule_id
		JOIN clients   c ON c.id = v.client_id`

	var args []interface{}
	if statut != "" && statut != "tous" {
		q += " WHERE i.statut = ?"
		args = append(args, statut)
	}
	q += " ORDER BY i.date_entree DESC, i.id DESC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Intervention
	for rows.Next() {
		var inv Intervention
		rows.Scan(&inv.ID, &inv.VehiculeID, &inv.Immatriculation,
			&inv.ClientNom, &inv.ClientTel, &inv.ClientAdresse,
			&inv.Marque, &inv.Modele,
			&inv.DateEntree, &inv.DateSortie, &inv.Description,
			&inv.Diagnostic, &inv.TravauxEffectues, &inv.PiecesUtilisees,
			&inv.CoutMainOeuvre, &inv.CoutPieces, &inv.MontantTotal,
			&inv.Statut, &inv.CreatedAt)
		list = append(list, inv)
	}
	return list, nil
}

func getInterventionByID(id int64) (Intervention, error) {
	var inv Intervention
	err := db.QueryRow(`
		SELECT i.id, i.vehicule_id, v.immatriculation,
		       c.nom||' '||c.prenom, COALESCE(c.telephone,''), COALESCE(c.adresse,''),
		       v.marque, v.modele,
		       i.date_entree, COALESCE(i.date_sortie,''), i.description,
		       COALESCE(i.diagnostic,''), COALESCE(i.travaux_effectues,''),
		       COALESCE(i.pieces_utilisees,''),
		       i.cout_main_oeuvre, i.cout_pieces, i.montant_total,
		       i.statut, i.created_at
		FROM interventions i
		JOIN vehicules v ON v.id = i.vehicule_id
		JOIN clients   c ON c.id = v.client_id
		WHERE i.id = ?`, id,
	).Scan(&inv.ID, &inv.VehiculeID, &inv.Immatriculation,
		&inv.ClientNom, &inv.ClientTel, &inv.ClientAdresse,
		&inv.Marque, &inv.Modele,
		&inv.DateEntree, &inv.DateSortie, &inv.Description,
		&inv.Diagnostic, &inv.TravauxEffectues, &inv.PiecesUtilisees,
		&inv.CoutMainOeuvre, &inv.CoutPieces, &inv.MontantTotal,
		&inv.Statut, &inv.CreatedAt)
	return inv, err
}

func createIntervention(vehiculeID int64, dateEntree, description string, coutMO, coutPieces float64) (int64, error) {
	total := coutMO + coutPieces
	r, err := db.Exec(`
		INSERT INTO interventions
		  (vehicule_id, date_entree, description, cout_main_oeuvre, cout_pieces, montant_total)
		VALUES (?, ?, ?, ?, ?, ?)`,
		vehiculeID, dateEntree, description, coutMO, coutPieces, total,
	)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func updateIntervention(id int64, desc, diag, travaux, pieces string, coutMO, coutPieces float64, statut string) error {
	total := coutMO + coutPieces
	var extra string
	if statut == "livre" || statut == "termine" {
		extra = ", date_sortie = date('now')"
	}
	_, err := db.Exec(`
		UPDATE interventions
		SET description=?, diagnostic=?, travaux_effectues=?, pieces_utilisees=?,
		    cout_main_oeuvre=?, cout_pieces=?, montant_total=?, statut=?`+extra+`
		WHERE id=?`,
		desc, diag, travaux, pieces, coutMO, coutPieces, total, statut, id,
	)
	return err
}

func updateStatutIntervention(id int64, statut string) error {
	if statut == "livre" || statut == "termine" {
		_, err := db.Exec(
			"UPDATE interventions SET statut=?, date_sortie=date('now') WHERE id=?",
			statut, id,
		)
		return err
	}
	_, err := db.Exec("UPDATE interventions SET statut=? WHERE id=?", statut, id)
	return err
}

func deleteIntervention(id int64) error {
	_, err := db.Exec("DELETE FROM interventions WHERE id = ?", id)
	return err
}

// ─── Paramètres du garage ────────────────────────────────────────────────────

// defaultSettings contient les valeurs initiales affichées sur les factures.
var defaultSettings = map[string]string{
	"nom_garage":       "Gorage",
	"adresse_garage":   "Votre adresse, Wilaya",
	"telephone_garage": "0XXX XXX XXX",
	"ville_garage":     "Alger — Algérie",
	// Informations fiscales obligatoires sur les factures
	"nif": "", // Numéro d'Identification Fiscale
	"nis": "", // Numéro d'Identification Statistique
	"rc":  "", // Registre de Commerce
	"ai":  "", // Article d'Imposition
	// Facturation
	"tva_taux_defaut":       "19",
	"mentions_legales":      "",
	"delai_validite_devis":  "30",
	// Atelier
	"heure_ouverture":  "08",
	"heure_fermeture":  "18",
	// Email SMTP
	"smtp_host":      "",
	"smtp_port":      "587",
	"smtp_user":      "",
	"smtp_password":  "",
	"smtp_from_name":  "",
	"smtp_from_email": "",
	// Sauvegarde cloud (S3 / R2 / B2)
	"s3_endpoint":   "",
	"s3_bucket":     "",
	"s3_access_key": "",
	"s3_secret_key": "",
	"s3_region":     "auto",
}

func getSettings() map[string]string {
	s := make(map[string]string)
	for k, v := range defaultSettings {
		s[k] = v
	}
	rows, err := db.Query("SELECT cle, valeur FROM settings")
	if err != nil {
		return s
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil && v != "" {
			s[k] = v
		}
	}
	return s
}

func updateSettings(data map[string]string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for k, v := range data {
		if _, err = tx.Exec(
			"INSERT INTO settings (cle, valeur) VALUES (?,?) ON CONFLICT(cle) DO UPDATE SET valeur=excluded.valeur",
			k, v,
		); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}
	}
	return tx.Commit()
}

// ─── Rendez-vous ─────────────────────────────────────────────────────────────

const rdvSelectBase = `
	SELECT r.id, r.client_id, r.vehicule_id,
	       c.nom||' '||c.prenom, COALESCE(c.telephone,''),
	       v.immatriculation, v.marque, v.modele,
	       r.date_rdv, r.heure_rdv, r.duree_min,
	       r.motif, COALESCE(r.notes,''), r.statut, r.created_at
	FROM rendezvous r
	JOIN clients  c ON c.id = r.client_id
	JOIN vehicules v ON v.id = r.vehicule_id`

func scanRDV(rows interface{ Scan(...interface{}) error }) (RendezVous, error) {
	var rdv RendezVous
	err := rows.Scan(
		&rdv.ID, &rdv.ClientID, &rdv.VehiculeID,
		&rdv.ClientNom, &rdv.ClientTel,
		&rdv.Immatriculation, &rdv.Marque, &rdv.Modele,
		&rdv.DateRDV, &rdv.HeureRDV, &rdv.DureeMin,
		&rdv.Motif, &rdv.Notes, &rdv.Statut, &rdv.CreatedAt,
	)
	return rdv, err
}

func getRDVsByPeriod(dateDebut, dateFin string) ([]RendezVous, error) {
	rows, err := db.Query(rdvSelectBase+
		` WHERE r.date_rdv BETWEEN ? AND ?
		  ORDER BY r.date_rdv, r.heure_rdv`,
		dateDebut, dateFin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RendezVous
	for rows.Next() {
		rdv, err := scanRDV(rows)
		if err == nil {
			list = append(list, rdv)
		}
	}
	return list, nil
}

func getRDVsByStatut(statut string) ([]RendezVous, error) {
	q := rdvSelectBase
	var args []interface{}
	if statut != "" {
		q += " WHERE r.statut = ?"
		args = append(args, statut)
	}
	q += " ORDER BY r.date_rdv, r.heure_rdv"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RendezVous
	for rows.Next() {
		rdv, err := scanRDV(rows)
		if err == nil {
			list = append(list, rdv)
		}
	}
	return list, nil
}

func getRDVsAujourdhui() ([]RendezVous, error) {
	rows, err := db.Query(rdvSelectBase+
		` WHERE r.date_rdv = date('now') AND r.statut != 'annule'
		  ORDER BY r.heure_rdv`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RendezVous
	for rows.Next() {
		rdv, err := scanRDV(rows)
		if err == nil {
			list = append(list, rdv)
		}
	}
	return list, nil
}

func getRDVByID(id int64) (RendezVous, error) {
	row := db.QueryRow(rdvSelectBase+" WHERE r.id = ?", id)
	return scanRDV(row)
}

func createRDV(clientID, vehiculeID int64, dateRDV, heureRDV string, dureeMin int, motif, notes string) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO rendezvous (client_id, vehicule_id, date_rdv, heure_rdv, duree_min, motif, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		clientID, vehiculeID, dateRDV, heureRDV, dureeMin, motif, notes,
	)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func updateRDVStatut(id int64, statut string) error {
	_, err := db.Exec("UPDATE rendezvous SET statut=? WHERE id=?", statut, id)
	return err
}

func deleteRDV(id int64) error {
	_, err := db.Exec("DELETE FROM rendezvous WHERE id=?", id)
	return err
}

// ─── Recherche globale ───────────────────────────────────────────────────────

type SearchResults struct {
	Query     string
	Clients   []Client
	Vehicules []Vehicule
}

func searchGlobal(q string) SearchResults {
	res := SearchResults{Query: q}
	if q == "" {
		return res
	}
	like := "%" + q + "%"

	rows, _ := db.Query(`
		SELECT id, nom, prenom, telephone, email, adresse, created_at
		FROM clients
		WHERE nom LIKE ? OR prenom LIKE ? OR telephone LIKE ? OR (nom||' '||prenom) LIKE ?
		ORDER BY nom, prenom`, like, like, like, like)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var c Client
			rows.Scan(&c.ID, &c.Nom, &c.Prenom, &c.Telephone, &c.Email, &c.Adresse, &c.CreatedAt)
			res.Clients = append(res.Clients, c)
		}
	}

	rows2, _ := db.Query(`
		SELECT v.id, v.client_id, c.nom||' '||c.prenom,
		       v.marque, v.modele, v.annee, v.immatriculation,
		       v.vin, v.kilometrage, v.created_at
		FROM vehicules v
		JOIN clients c ON c.id = v.client_id
		WHERE v.immatriculation LIKE ? OR v.marque LIKE ? OR v.modele LIKE ? OR v.vin LIKE ?
		ORDER BY v.immatriculation`, like, like, like, like)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var v Vehicule
			rows2.Scan(&v.ID, &v.ClientID, &v.ClientNom, &v.Marque, &v.Modele,
				&v.Annee, &v.Immatriculation, &v.VIN, &v.Kilometrage, &v.CreatedAt)
			res.Vehicules = append(res.Vehicules, v)
		}
	}

	return res
}

// ─── Pièces / Stock ──────────────────────────────────────────────────────────

func getPieces(search, categorie string) ([]Piece, error) {
	q := `SELECT id, reference, nom, categorie, quantite_stock,
	             seuil_alerte, prix_achat, prix_vente, fournisseur, notes, created_at
	      FROM pieces`
	var args []interface{}
	var conds []string
	if search != "" {
		conds = append(conds, "(nom LIKE ? OR reference LIKE ? OR fournisseur LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	if categorie != "" {
		conds = append(conds, "categorie = ?")
		args = append(args, categorie)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY nom"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Piece
	for rows.Next() {
		var p Piece
		rows.Scan(&p.ID, &p.Reference, &p.Nom, &p.Categorie,
			&p.QuantiteStock, &p.SeuilAlerte,
			&p.PrixAchat, &p.PrixVente,
			&p.Fournisseur, &p.Notes, &p.CreatedAt)
		p.StockBas = p.QuantiteStock <= p.SeuilAlerte
		list = append(list, p)
	}
	return list, nil
}

func getPieceByID(id int64) (Piece, error) {
	var p Piece
	err := db.QueryRow(`SELECT id, reference, nom, categorie, quantite_stock,
	             seuil_alerte, prix_achat, prix_vente, fournisseur, notes, created_at
	      FROM pieces WHERE id = ?`, id,
	).Scan(&p.ID, &p.Reference, &p.Nom, &p.Categorie,
		&p.QuantiteStock, &p.SeuilAlerte,
		&p.PrixAchat, &p.PrixVente,
		&p.Fournisseur, &p.Notes, &p.CreatedAt)
	p.StockBas = p.QuantiteStock <= p.SeuilAlerte
	return p, err
}

func createPiece(ref, nom, cat string, qte, seuil int64, prixAchat, prixVente float64, fourn, notes string) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO pieces (reference, nom, categorie, quantite_stock, seuil_alerte,
		                    prix_achat, prix_vente, fournisseur, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, nom, cat, qte, seuil, prixAchat, prixVente, fourn, notes,
	)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func updatePiece(id int64, ref, nom, cat string, qte, seuil int64, prixAchat, prixVente float64, fourn, notes string) error {
	_, err := db.Exec(`
		UPDATE pieces SET reference=?, nom=?, categorie=?, quantite_stock=?, seuil_alerte=?,
		                  prix_achat=?, prix_vente=?, fournisseur=?, notes=?
		WHERE id=?`,
		ref, nom, cat, qte, seuil, prixAchat, prixVente, fourn, notes, id,
	)
	return err
}

func ajusterStock(id int64, delta int64) error {
	_, err := db.Exec(
		"UPDATE pieces SET quantite_stock = MAX(0, quantite_stock + ?) WHERE id = ?",
		delta, id,
	)
	return err
}

func deletePiece(id int64) error {
	_, err := db.Exec("DELETE FROM pieces WHERE id=?", id)
	return err
}

func getCategories() ([]string, error) {
	rows, err := db.Query(
		"SELECT DISTINCT categorie FROM pieces WHERE categorie != '' ORDER BY categorie",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var c string
		rows.Scan(&c)
		list = append(list, c)
	}
	return list, nil
}

// ─── Statistiques ────────────────────────────────────────────────────────────

type MoisStat struct {
	Label string
	Count int
	Pct   float64
}

type PieceStat struct {
	Designation string
	Total       float64
	Pct         float64
}

func getStatsInterventionsParMois() []MoisStat {
	// Initialiser les 12 derniers mois à zéro
	now := time.Now()
	counts := make(map[string]int)
	order  := make([]string, 12)
	labels := make(map[string]string)
	moisFR := []string{"Jan","Fév","Mar","Avr","Mai","Jun","Jul","Aoû","Sep","Oct","Nov","Déc"}
	for i := 11; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		key := t.Format("2006-01")
		order[11-i] = key
		counts[key] = 0
		labels[key] = moisFR[t.Month()-1] + " " + t.Format("06")
	}
	rows, err := db.Query(`
		SELECT strftime('%Y-%m', date_entree) as m, COUNT(*)
		FROM interventions
		WHERE date_entree >= date('now','-11 months','start of month')
		GROUP BY m`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m string
			var n int
			rows.Scan(&m, &n)
			counts[m] = n
		}
	}
	// Calculer le max pour les pourcentages
	var maxV int
	for _, k := range order {
		if counts[k] > maxV {
			maxV = counts[k]
		}
	}
	if maxV == 0 {
		maxV = 1
	}
	result := make([]MoisStat, 12)
	for i, k := range order {
		result[i] = MoisStat{
			Label: labels[k],
			Count: counts[k],
			Pct:   float64(counts[k]) / float64(maxV) * 100,
		}
	}
	return result
}

func getStatsPiecesTop() []PieceStat {
	rows, err := db.Query(`
		SELECT designation, SUM(quantite) as total
		FROM (
			SELECT TRIM(designation) as designation, quantite FROM devis_lignes    WHERE type='piece'
			UNION ALL
			SELECT TRIM(designation) as designation, quantite FROM factures_lignes WHERE type='piece'
		)
		GROUP BY LOWER(TRIM(designation))
		ORDER BY total DESC
		LIMIT 10`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var list []PieceStat
	var maxV float64
	for rows.Next() {
		var p PieceStat
		rows.Scan(&p.Designation, &p.Total)
		if p.Total > maxV {
			maxV = p.Total
		}
		list = append(list, p)
	}
	if maxV == 0 {
		maxV = 1
	}
	for i := range list {
		list[i].Pct = list[i].Total / maxV * 100
	}
	return list
}

// ─── Tableau de bord ─────────────────────────────────────────────────────────

func getStats() Stats {
	var s Stats
	db.QueryRow("SELECT COUNT(*) FROM clients").Scan(&s.TotalClients)
	db.QueryRow("SELECT COUNT(*) FROM vehicules").Scan(&s.TotalVehicules)
	db.QueryRow(`SELECT COUNT(*) FROM interventions
	             WHERE statut IN ('en_cours','attente_pieces')`).Scan(&s.InterventionsActives)
	db.QueryRow(`SELECT COALESCE(SUM(montant_total),0) FROM interventions
	             WHERE strftime('%Y-%m', date_entree) = strftime('%Y-%m','now')`).Scan(&s.CAMois)
	db.QueryRow(`SELECT COUNT(*) FROM rendezvous
	             WHERE date_rdv = date('now') AND statut != 'annule'`).Scan(&s.RDVAujourdhui)
	db.QueryRow(
		`SELECT COUNT(*) FROM pieces WHERE quantite_stock <= seuil_alerte`,
	).Scan(&s.PiecesStockBas)
	return s
}
