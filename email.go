package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	gomail "gopkg.in/mail.v2"
)

// sendEmail envoie un email avec une pièce jointe PDF générée en mémoire.
func sendEmail(to, subject, body string, pdfName string, pdfBuf *bytes.Buffer) error {
	gs := getSettings()

	host     := gs["smtp_host"]
	portStr  := gs["smtp_port"]
	user     := gs["smtp_user"]
	pass     := gs["smtp_password"]
	fromName := gs["smtp_from_name"]
	fromAddr := gs["smtp_from_email"]

	if host == "" || user == "" {
		return fmt.Errorf("configuration SMTP incomplète — vérifiez les paramètres Email")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		port = 587
	}

	if fromAddr == "" {
		fromAddr = user
	}
	from := fromAddr
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromAddr)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)
	m.Attach(pdfName, gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := w.Write(pdfBuf.Bytes())
		return err
	}))

	d := gomail.NewDialer(host, port, user, pass)
	return d.DialAndSend(m)
}

// ── Handler : envoyer devis par email ────────────────────────────────────────

func devisEmailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	to    := strings.TrimSpace(r.FormValue("email"))
	msg   := strings.TrimSpace(r.FormValue("message"))

	if to == "" {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Adresse+email+manquante", id), http.StatusFound)
		return
	}

	dv, err := getDevisByID(id)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Devis+introuvable", id), http.StatusFound)
		return
	}

	// Générer le PDF en mémoire
	gs := getSettings()
	isOR := dv.Statut == "accepte"
	pdf := buildDevisPDF(dv, gs, isOR)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Erreur+génération+PDF", id), http.StatusFound)
		return
	}

	if msg == "" {
		msg = fmt.Sprintf("Bonjour,\n\nVeuillez trouver ci-joint votre devis %s.\n\nCordialement,\n%s",
			dv.Numero, gs["nom_garage"])
	}

	subject := fmt.Sprintf("Devis %s — %s", dv.Numero, gs["nom_garage"])
	if err := sendEmail(to, subject, msg, dv.Numero+".pdf", &buf); err != nil {
		log.Println("devisEmail:", err)
		http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&err=Erreur+lors+de+l'envoi+de+l'email", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/devis/view?id=%d&ok=Email+envoyé+à+%s", id, to), http.StatusFound)
}

// ── Handler : envoyer facture par email ──────────────────────────────────────

func factureEmailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	to    := strings.TrimSpace(r.FormValue("email"))
	msg   := strings.TrimSpace(r.FormValue("message"))

	if to == "" {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Adresse+email+manquante", id), http.StatusFound)
		return
	}

	f, err := getFactureByID(id)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Facture+introuvable", id), http.StatusFound)
		return
	}

	// Générer le PDF en mémoire
	pdf := buildFactureLegalePDF(f)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Erreur+génération+PDF", id), http.StatusFound)
		return
	}

	gs := getSettings()
	if msg == "" {
		msg = fmt.Sprintf("Bonjour,\n\nVeuillez trouver ci-joint votre facture %s.\n\nCordialement,\n%s",
			f.Numero, gs["nom_garage"])
	}

	subject := fmt.Sprintf("Facture %s — %s", f.Numero, gs["nom_garage"])
	if err := sendEmail(to, subject, msg, f.Numero+".pdf", &buf); err != nil {
		log.Println("factureEmail:", err)
		http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&err=Erreur+lors+de+l'envoi+de+l'email", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/factures/view?id=%d&ok=Email+envoyé+à+%s", id, to), http.StatusFound)
}
