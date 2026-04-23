package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"
)

// ─── Handler HTTP ─────────────────────────────────────────────────────────────

func factureLegalePrintHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	f, err := getFactureByID(id)
	if err != nil {
		http.Redirect(w, r, "/factures?err=Facture+introuvable", http.StatusFound)
		return
	}
	pdf := buildFactureLegalePDF(f)
	w.Header().Set("Content-Type", "application/pdf")
	disp := "inline"
	if r.URL.Query().Get("dl") == "1" {
		disp = "attachment"
	}
	w.Header().Set("Content-Disposition", disp+`; filename="`+f.Numero+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		log.Println("factureLegalePDF:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
	}
}

// ─── Construction PDF (conforme loi anti-fraude algérienne) ──────────────────

func buildFactureLegalePDF(f FactureOfficielle) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 30)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pageW, pageH := pdf.GetPageSize()
	ml, mt, mr, mb := pdf.GetMargins()
	cw := pageW - ml - mr

	fill := func(c rgb) { pdf.SetFillColor(c.r, c.g, c.b) }
	draw := func(c rgb) { pdf.SetDrawColor(c.r, c.g, c.b) }
	txt  := func(c rgb) { pdf.SetTextColor(c.r, c.g, c.b) }
	lw   := func(v float64) { pdf.SetLineWidth(v) }

	// ════════════════════════════════════════════════════
	// SECTION 1 — EN-TÊTE
	// ════════════════════════════════════════════════════

	leftW  := cw * 0.50
	rightW := cw * 0.46

	// Gauche : titre FACTURE/AVOIR + statut + ref/date
	titre := "FACTURE"
	if f.TypeDoc == "avoir" {
		titre = "AVOIR"
	}
	txt(cDark)
	pdf.SetFont("Arial", "B", 34)
	pdf.SetXY(ml, mt)
	pdf.Cell(leftW, 14, titre)

	statutLabels := map[string]string{
		"emise": "ÉMISE", "payee": "PAYÉE", "annulee": "ANNULÉE",
	}
	pdf.SetFont("Arial", "B", 10)
	txt(cTextLt)
	pdf.SetXY(ml, mt+16)
	pdf.Cell(leftW, 5, tr(statutLabels[f.Statut]))

	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	pdf.SetXY(ml, mt+23)
	pdf.Cell(leftW, 5, tr("Réf : "+f.Numero))
	if f.TypeDoc == "avoir" && f.AvoirPourNumero != "" {
		pdf.SetXY(ml, mt+29)
		pdf.Cell(leftW, 5, tr("Avoir de : "+f.AvoirPourNumero))
		pdf.SetXY(ml, mt+35)
		pdf.Cell(leftW, 5, tr("Émission : "+fmtDatePDF(f.DateEmission)))
	} else {
		pdf.SetXY(ml, mt+29)
		pdf.Cell(leftW, 5, tr("Émission : "+fmtDatePDF(f.DateEmission)))
		if f.DateEcheance != "" {
			pdf.SetXY(ml, mt+35)
			pdf.Cell(leftW, 5, tr("Échéance : "+fmtDatePDF(f.DateEcheance)))
		}
	}

	// Droite : bloc garage (fond gris clair, bordure)
	bx := ml + leftW + cw*0.04
	bH := 44.0
	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)
	pdf.Rect(bx, mt-1, rightW, bH, "FD")

	txt(cDark)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(bx+4, mt+3)
	pdf.Cell(rightW-8, 6, tr(f.GarageNom))

	pdf.SetFont("Arial", "", 8)
	txt(cTextLt)
	pdf.SetXY(bx+4, mt+11)
	addr := f.GarageAdresse
	if f.GarageVille != "" {
		addr += "\n" + f.GarageVille
	}
	if f.GarageTelephone != "" {
		addr += "\n" + f.GarageTelephone
	}
	pdf.MultiCell(rightW-8, 4, tr(addr), "", "L", false)

	// Identifiants fiscaux du garage dans le bloc
	if f.GarageNIF != "" || f.GarageNIS != "" || f.GarageRC != "" || f.GarageAI != "" {
		fiscLineY := mt + 28
		pdf.SetFont("Arial", "", 7.5)
		txt(cTextLt)
		pdf.SetXY(bx+4, fiscLineY)
		pdf.MultiCell(rightW-8, 3.5,
			tr("NIF:"+notEmpty(f.GarageNIF, "—")+
				"  NIS:"+notEmpty(f.GarageNIS, "—")+
				"  RC:"+notEmpty(f.GarageRC, "—")+
				"  AI:"+notEmpty(f.GarageAI, "—")),
			"", "L", false)
	}

	pdf.SetY(mt + bH + 6)

	// ════════════════════════════════════════════════════
	// SECTION 2 — CLIENT + VÉHICULE
	// ════════════════════════════════════════════════════

	infoY := pdf.GetY()
	hw    := (cw - 6) / 2
	infoH := 32.0

	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)

	// Client
	pdf.Rect(ml, infoY, hw, infoH, "FD")
	pdf.SetFont("Arial", "B", 8)
	txt(cTextLt)
	pdf.SetXY(ml+4, infoY+3)
	pdf.Cell(hw-8, 4, tr("FACTURÉ À"))
	pdf.SetFont("Arial", "B", 11)
	txt(cDark)
	pdf.SetXY(ml+4, infoY+9)
	pdf.Cell(hw-8, 6, tr(f.ClientNom))
	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	if f.ClientTelephone != "" {
		pdf.SetXY(ml+4, infoY+16)
		pdf.Cell(hw-8, 4, tr("Tél : "+f.ClientTelephone))
	}
	if f.ClientAdresse != "" {
		pdf.SetXY(ml+4, infoY+21)
		pdf.MultiCell(hw-8, 4, tr(f.ClientAdresse), "", "L", false)
	}
	if f.ClientNIF != "" {
		pdf.SetXY(ml+4, infoY+26)
		pdf.Cell(hw-8, 4, tr("NIF client : "+f.ClientNIF))
	}

	// Véhicule + dates
	vx := ml + hw + 6
	pdf.Rect(vx, infoY, hw, infoH, "FD")
	pdf.SetFont("Arial", "B", 8)
	txt(cTextLt)
	pdf.SetXY(vx+4, infoY+3)
	pdf.Cell(hw-8, 4, tr("VÉHICULE & DATES"))
	pdf.SetFont("Arial", "B", 12)
	txt(cDark)
	pdf.SetXY(vx+4, infoY+9)
	pdf.Cell(hw-8, 7, tr(f.VehiculeImmat))
	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	if f.VehiculeMarque != "" || f.VehiculeModele != "" {
		pdf.SetXY(vx+4, infoY+17)
		pdf.Cell(hw-8, 4, tr(f.VehiculeMarque+" "+f.VehiculeModele))
	}
	pdf.SetXY(vx+4, infoY+22)
	pdf.Cell(hw-8, 4, tr("Émission : "+fmtDatePDF(f.DateEmission)))
	if f.DateEcheance != "" {
		pdf.SetXY(vx+4, infoY+27)
		pdf.Cell(hw-8, 4, tr("Échéance : "+fmtDatePDF(f.DateEcheance)))
	}

	pdf.SetY(infoY + infoH + 8)

	// ════════════════════════════════════════════════════
	// SECTION 3 — TABLEAU DES PRESTATIONS
	// ════════════════════════════════════════════════════

	tableY := pdf.GetY()
	col  := []float64{cw * 0.09, cw * 0.33, cw * 0.07, cw * 0.11, cw * 0.07, cw * 0.11, cw * 0.11, cw * 0.11}
	hdrs := []string{"Type", tr("Désignation"), tr("Qté"), "P.U. HT", "TVA%", "Total HT", "TVA", "TTC"}

	fill(cDark)
	txt(cWhite)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(ml, tableY)
	for i, h := range hdrs {
		align := "L"
		if i > 1 {
			align = "R"
		}
		pdf.CellFormat(col[i], 7, tr(h), "0", 0, align, true, 0, "")
	}
	pdf.Ln(7)

	odd := true
	for _, l := range f.Lignes {
		bg := cWhite
		if odd {
			bg = cGrayBg
		}
		odd = !odd
		fill(bg)
		txt(cDark)
		y0 := pdf.GetY()
		pdf.SetFont("Arial", "", 8)
		pdf.SetXY(ml, y0)
		pdf.CellFormat(col[0], 6, tr(ligneTypeLabel(l.Type)), "0", 0, "L", true, 0, "")

		startX := ml + col[0]
		pdf.SetXY(startX, y0)
		pdf.MultiCell(col[1], 6, tr(l.Designation), "0", "L", true)
		endY := pdf.GetY()
		rowH := endY - y0

		pdf.SetXY(startX+col[1], y0)
		pdf.CellFormat(col[2], rowH, fmt.Sprintf("%.2f", l.Quantite), "0", 0, "R", true, 0, "")
		pdf.CellFormat(col[3], rowH, fmt.Sprintf("%.2f", l.PrixUnitHT), "0", 0, "R", true, 0, "")
		pdf.CellFormat(col[4], rowH, fmt.Sprintf("%.0f%%", l.TVATaux), "0", 0, "R", true, 0, "")
		pdf.CellFormat(col[5], rowH, fmt.Sprintf("%.2f", l.MontantHT), "0", 0, "R", true, 0, "")
		pdf.CellFormat(col[6], rowH, fmt.Sprintf("%.2f", l.MontantTVA), "0", 0, "R", true, 0, "")
		pdf.CellFormat(col[7], rowH, fmt.Sprintf("%.2f", l.MontantTTC), "0", 1, "R", true, 0, "")
	}

	tableEndY := pdf.GetY()
	draw(cGrayBdr)
	lw(0.3)
	pdf.Rect(ml, tableY+7, cw, tableEndY-tableY-7, "D")

	// ════════════════════════════════════════════════════
	// SECTION 4 — RÉCAPITULATIF FINANCIER
	// ════════════════════════════════════════════════════

	pdf.Ln(5)
	totW := 90.0
	totX := ml + cw - totW

	for _, row := range []struct {
		label string
		val   float64
	}{
		{tr("Base HT :"), f.MontantHT},
		{tr(fmt.Sprintf("TVA %.0f%% :", f.TVATaux)), f.MontantTVA},
	} {
		txt(cTextLt)
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(totX, pdf.GetY())
		pdf.Cell(totW*0.55, 6, row.label)
		txt(cDark)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(totW*0.45, 6, fmt.Sprintf("%.2f DZD", row.val))
		pdf.Ln(6)
	}

	lineY := pdf.GetY()
	draw(cGrayBdr)
	lw(0.5)
	pdf.Line(totX, lineY, ml+cw, lineY)
	pdf.Ln(4)

	// Bloc TTC (fond noir)
	fill(cDark)
	pdf.Rect(totX, pdf.GetY(), totW, 14, "F")
	txt(cWhite)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(totX+4, pdf.GetY()+3)
	pdf.Cell(totW*0.5, 8, tr("NET À PAYER TTC"))
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(totW*0.5-4, 8, fmt.Sprintf("%.2f DZD", f.MontantTTC))
	pdf.Ln(18)

	// Mode de règlement
	modes := map[string]string{
		"especes": "Espèces", "cheque": "Chèque", "virement": "Virement bancaire",
	}
	modeLabel := modes[f.ModeReglement]
	if modeLabel == "" {
		modeLabel = f.ModeReglement
	}
	txt(cTextLt)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(ml, pdf.GetY())
	pdf.Cell(cw, 5, tr("Mode de règlement : "+modeLabel))
	pdf.Ln(8)

	// ════════════════════════════════════════════════════
	// SECTION 5 — ANNULATION (si applicable)
	// ════════════════════════════════════════════════════

	if f.Statut == "annulee" && f.MotifAnnulation != "" {
		y0 := pdf.GetY()
		fill(cGrayBg)
		draw(cGrayBdr)
		lw(0.5)
		pdf.Rect(ml, y0, cw, 12, "FD")
		txt(cDark)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(ml+4, y0+2)
		pdf.Cell(cw-8, 4, tr("FACTURE ANNULÉE — Motif : "+f.MotifAnnulation))
		pdf.Ln(14)
	}

	// ════════════════════════════════════════════════════
	// SECTION 6 — MENTIONS LÉGALES + PIED DE PAGE
	// ════════════════════════════════════════════════════

	gs := getSettings()
	if m := gs["mentions_legales"]; m != "" {
		footMentionsH := 10.0
		mentionsY := pageH - mb - 22 - footMentionsH - 4
		txt(cTextLt)
		pdf.SetFont("Arial", "I", 7.5)
		pdf.SetXY(ml, mentionsY)
		pdf.MultiCell(cw, 3.8, tr(m), "", "L", false)
	}

	footY := pageH - mb - 22
	draw(cGrayBdr)
	lw(0.5)
	pdf.Line(ml, footY, ml+cw, footY)

	txt(cTextLt)
	pdf.SetFont("Arial", "", 7.5)
	pdf.SetXY(ml, footY+3)
	pdf.MultiCell(cw*0.65, 4,
		tr(f.GarageNom+" | "+f.GarageAdresse+" | "+f.GarageVille+
			"\nNIF : "+notEmpty(f.GarageNIF, "—")+
			" | NIS : "+notEmpty(f.GarageNIS, "—")+
			" | RC : "+notEmpty(f.GarageRC, "—")+
			" | AI : "+notEmpty(f.GarageAI, "—")),
		"", "L", false)

	// Hash anti-fraude (droite)
	txt(rgb{148, 163, 184})
	pdf.SetFont("Arial", "", 6.5)
	pdf.SetXY(ml+cw*0.67, footY+3)
	pdf.MultiCell(cw*0.33, 3.5,
		tr("Empreinte : "+f.HashControle+
			"\nDoc. conforme \u2014 Toute modification invalide ce document."),
		"", "R", false)

	// Filigrane ANNULÉE ou AVOIR (diagonal, gris clair)
	filigrane := ""
	if f.Statut == "annulee" {
		filigrane = "ANNULÉE"
	} else if f.TypeDoc == "avoir" {
		filigrane = "AVOIR"
	}
	if filigrane != "" {
		pdf.SetFont("Arial", "B", 60)
		txt(rgb{200, 200, 200})
		pdf.TransformBegin()
		pdf.TransformRotate(45, pageW/2, pageH/2)
		pdf.SetXY(pageW/2-50, pageH/2-15)
		pdf.Cell(100, 30, tr(filigrane))
		pdf.TransformEnd()
	}

	_ = mr

	return pdf
}

// notEmpty retourne val si non vide, sinon fallback.
func notEmpty(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}
