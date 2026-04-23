package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"
)

// ─── Palette de couleurs ─────────────────────────────────────────────────────

type rgb struct{ r, g, b int }

var (
	cPrimary = rgb{79, 70, 229}   // #4f46e5 indigo
	cDark    = rgb{30, 41, 59}    // #1e293b slate-900
	cGrayBg  = rgb{248, 250, 252} // #f8fafc
	cGrayBdr = rgb{226, 232, 240} // #e2e8f0
	cTextLt  = rgb{100, 116, 139} // #64748b slate-500
	cWhite   = rgb{255, 255, 255}
	cGreen   = rgb{22, 163, 74}   // #16a34a
)

// ─── Handler HTTP ─────────────────────────────────────────────────────────────

func factureHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id == 0 {
		http.Redirect(w, r, "/interventions", http.StatusFound)
		return
	}

	inv, err := getInterventionByID(id)
	if err != nil {
		http.Redirect(w, r, "/interventions?err=Intervention+introuvable", http.StatusFound)
		return
	}

	gs := getSettings()
	pdf := buildFacturePDF(inv, gs)

	filename := fmt.Sprintf("facture-%04d.pdf", id)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)

	if err := pdf.Output(w); err != nil {
		log.Println("facturePDF:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
	}
}

// ─── Construction du PDF ─────────────────────────────────────────────────────

func buildFacturePDF(inv Intervention, gs map[string]string) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 25)
	pdf.AddPage()

	// Encodage Latin-1 pour les caractères français (é à ç ...)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pageW, pageH := pdf.GetPageSize()
	ml, mt, mr, mb := pdf.GetMargins()
	cw := pageW - ml - mr // 180 mm

	// ── helpers inline ──
	fill := func(c rgb) { pdf.SetFillColor(c.r, c.g, c.b) }
	draw := func(c rgb) { pdf.SetDrawColor(c.r, c.g, c.b) }
	txt  := func(c rgb) { pdf.SetTextColor(c.r, c.g, c.b) }
	lw   := func(w float64) { pdf.SetLineWidth(w) }

	// ════════════════════════════════════════════════════════
	// SECTION 1 — EN-TÊTE
	// ════════════════════════════════════════════════════════

	// ── Gauche : titre FACTURE ──
	txt(cPrimary)
	pdf.SetFont("Arial", "B", 34)
	pdf.SetXY(ml, mt)
	pdf.Cell(cw*0.52, 13, "FACTURE")

	pdf.SetFont("Arial", "", 11)
	txt(cTextLt)
	pdf.SetXY(ml, mt+14)
	pdf.Cell(cw*0.52, 6, tr(fmt.Sprintf("N° FAC-%04d", inv.ID)))

	// Date de facture = date sortie si renseignée, sinon date entrée
	dateFacture := fmtDatePDF(inv.DateSortie)
	if dateFacture == "" {
		dateFacture = fmtDatePDF(inv.DateEntree)
	}
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(ml, mt+21)
	pdf.Cell(cw*0.52, 5, tr("Date : "+dateFacture))

	statLabel := map[string]string{
		"en_cours": "En cours", "attente_pieces": "Attente pièces",
		"termine": "Terminé", "livre": "Livré",
	}[inv.Statut]
	pdf.SetXY(ml, mt+27)
	txt(cGreen)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(cw*0.52, 5, tr("Statut : "+statLabel))

	headerEndY := mt + 34.0

	// ── Droite : bloc garage (fond indigo) ──
	bx := ml + cw*0.54
	bw := cw * 0.46
	fill(cPrimary)
	pdf.Rect(bx, mt-1, bw, headerEndY-mt+2, "F")

	txt(cWhite)
	pdf.SetFont("Arial", "B", 13)
	pdf.SetXY(bx+5, mt+3)
	pdf.Cell(bw-10, 8, tr(gs["nom_garage"]))

	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(bx+5, mt+13)
	pdf.MultiCell(bw-10, 5,
		tr(gs["adresse_garage"]+"\n"+gs["telephone_garage"]+"\n"+gs["ville_garage"]),
		"", "L", false)

	pdf.SetY(headerEndY + 7)

	// ════════════════════════════════════════════════════════
	// SECTION 2 — CLIENT + VÉHICULE
	// ════════════════════════════════════════════════════════

	infoY := pdf.GetY()
	infoH := 34.0
	hw    := (cw - 6) / 2

	draw(cGrayBdr)
	lw(0.3)

	// Bloc client (gauche)
	fill(cGrayBg)
	pdf.Rect(ml, infoY, hw, infoH, "FD")

	pdf.SetFont("Arial", "B", 8)
	txt(cTextLt)
	pdf.SetXY(ml+4, infoY+3)
	pdf.Cell(hw-8, 4, "CLIENT")

	pdf.SetFont("Arial", "B", 11)
	txt(cDark)
	pdf.SetXY(ml+4, infoY+9)
	pdf.Cell(hw-8, 6, tr(inv.ClientNom))

	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	if inv.ClientTel != "" {
		pdf.SetXY(ml+4, infoY+16)
		pdf.Cell(hw-8, 4, tr("Tél : "+inv.ClientTel))
	}
	if inv.ClientAdresse != "" {
		pdf.SetXY(ml+4, infoY+21)
		pdf.MultiCell(hw-8, 4, tr(inv.ClientAdresse), "", "L", false)
	}

	// Bloc véhicule (droite)
	vx := ml + hw + 6
	fill(cGrayBg)
	pdf.Rect(vx, infoY, hw, infoH, "FD")

	pdf.SetFont("Arial", "B", 8)
	txt(cTextLt)
	pdf.SetXY(vx+4, infoY+3)
	pdf.Cell(hw-8, 4, tr("VÉHICULE"))

	pdf.SetFont("Arial", "B", 14)
	txt(cDark)
	pdf.SetXY(vx+4, infoY+9)
	pdf.Cell(hw-8, 7, tr(inv.Immatriculation))

	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	pdf.SetXY(vx+4, infoY+17)
	pdf.Cell(hw-8, 4, tr(inv.Marque+" "+inv.Modele))
	pdf.SetXY(vx+4, infoY+22)
	pdf.Cell(hw-8, 4, tr("Entrée : "+fmtDatePDF(inv.DateEntree)))
	if inv.DateSortie != "" {
		pdf.SetXY(vx+4, infoY+27)
		pdf.Cell(hw-8, 4, tr("Sortie : "+fmtDatePDF(inv.DateSortie)))
	}

	pdf.SetY(infoY + infoH + 9)

	// ════════════════════════════════════════════════════════
	// SECTION 3 — TABLEAU DES PRESTATIONS
	// ════════════════════════════════════════════════════════

	tableY := pdf.GetY()
	c1     := cw * 0.72 // col désignation
	c2     := cw * 0.28 // col montant

	// En-tête du tableau
	fill(cDark)
	txt(cWhite)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(ml, tableY)
	pdf.CellFormat(c1, 8, tr("  Désignation"), "0", 0, "L", true, 0, "")
	pdf.CellFormat(c2, 8, "Montant (DZD)", "0", 1, "R", true, 0, "")

	// ── Ligne 1 : Main d'œuvre ──
	pdfTableRow(pdf, tr, ml, c1, c2,
		tr("Main d'œuvre"),
		inv.TravauxEffectues, // détail → travaux effectués en priorité
		inv.Description,      // fallback description
		inv.CoutMainOeuvre,
		cGrayBg, cWhite, cDark, cTextLt)

	// ── Ligne 2 : Pièces ──
	pdfTableRow(pdf, tr, ml, c1, c2,
		tr("Pièces et fournitures"),
		inv.PiecesUtilisees, "",
		inv.CoutPieces,
		cGrayBg, cWhite, cDark, cTextLt)

	tableEndY := pdf.GetY()

	// Bordure extérieure du tableau
	draw(cGrayBdr)
	lw(0.3)
	pdf.Rect(ml, tableY+8, cw, tableEndY-tableY-8, "D")

	// ════════════════════════════════════════════════════════
	// SECTION 4 — RÉCAPITULATIF & TOTAL
	// ════════════════════════════════════════════════════════

	pdf.Ln(6)
	totW := 82.0
	totX := ml + cw - totW

	// Lignes sous-totaux
	for _, row := range []struct {
		label  string
		amount float64
	}{
		{tr("Main d'œuvre :"), inv.CoutMainOeuvre},
		{tr("Pièces :"), inv.CoutPieces},
	} {
		txt(cTextLt)
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(totX, pdf.GetY())
		pdf.Cell(totW*0.6, 6, row.label)
		txt(cDark)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(totW*0.4, 6, fmtMoney(row.amount))
		pdf.Ln(6)
	}

	// Ligne séparatrice
	lineY := pdf.GetY()
	draw(cGrayBdr)
	lw(0.4)
	pdf.Line(totX, lineY, ml+cw, lineY)
	pdf.Ln(5)

	// Boîte TOTAL
	tBoxH := 14.0
	fill(cPrimary)
	pdf.Rect(totX, pdf.GetY(), totW, tBoxH, "F")
	txt(cWhite)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(totX+4, pdf.GetY()+3)
	pdf.Cell(totW*0.5, 8, "TOTAL TTC")
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(totW*0.5-4, 8, fmtMoney(inv.MontantTotal))

	pdf.Ln(tBoxH + 10)

	// ════════════════════════════════════════════════════════
	// SECTION 5 — DIAGNOSTIC / NOTES (optionnel)
	// ════════════════════════════════════════════════════════

	if inv.Diagnostic != "" {
		txt(cTextLt)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetX(ml)
		pdf.Cell(cw, 5, "NOTES & DIAGNOSTIC")
		pdf.Ln(5)

		fill(cGrayBg)
		draw(cGrayBdr)
		lw(0.3)
		txt(cDark)
		pdf.SetFont("Arial", "", 9)
		pdf.SetX(ml)
		pdf.MultiCell(cw, 5, tr(inv.Diagnostic), "1", "L", true)
		pdf.Ln(6)
	}

	// ════════════════════════════════════════════════════════
	// PIED DE PAGE (ancré en bas)
	// ════════════════════════════════════════════════════════

	footY := pageH - mb - 14
	draw(cPrimary)
	lw(0.5)
	pdf.Line(ml, footY, ml+cw, footY)

	txt(cTextLt)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetXY(ml, footY+5)
	pdf.CellFormat(cw, 5,
		tr(fmt.Sprintf("Merci de votre confiance — %s — %s — %s",
			gs["nom_garage"], gs["telephone_garage"], gs["ville_garage"])),
		"", 0, "C", false, 0, "")

	return pdf
}

// pdfTableRow dessine une ligne du tableau avec un sous-détail optionnel.
func pdfTableRow(
	pdf *fpdf.Fpdf, tr func(string) string,
	ml, c1, c2 float64,
	label, detail, fallback string,
	montant float64,
	bgRow, bgDetail, cDk, cLt rgb,
) {
	fill := func(c rgb) { pdf.SetFillColor(c.r, c.g, c.b) }
	txt  := func(c rgb) { pdf.SetTextColor(c.r, c.g, c.b) }

	// Ligne principale
	fill(bgRow)
	txt(cDk)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(ml, pdf.GetY())
	pdf.CellFormat(c1, 7, "  "+label, "0", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(c2, 7, tr(fmt.Sprintf("  %.2f", montant)), "0", 1, "R", true, 0, "")

	// Détail (italique, clair)
	d := detail
	if d == "" {
		d = fallback
	}
	if d != "" {
		startY := pdf.GetY()
		fill(bgDetail)
		txt(cLt)
		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(ml, startY)
		pdf.MultiCell(c1, 5, tr("    "+d), "0", "L", false)
		endY := pdf.GetY()
		// Remplir la colonne montant pour que la hauteur soit cohérente
		pdf.SetXY(ml+c1, startY)
		pdf.Rect(ml+c1, startY, c2, endY-startY, "F")
		pdf.SetXY(ml, endY)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// fmtDatePDF convertit YYYY-MM-DD → JJ/MM/AAAA (usage hors template).
func fmtDatePDF(s string) string {
	if len(s) < 10 {
		return ""
	}
	return s[8:10] + "/" + s[5:7] + "/" + s[0:4]
}

// fmtMoney formate un montant en DZD avec 2 décimales.
func fmtMoney(v float64) string {
	return fmt.Sprintf("%.2f DZD", v)
}
