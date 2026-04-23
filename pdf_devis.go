package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"
)

// ─── Handler HTTP ─────────────────────────────────────────────────────────────

func devisPrintHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	dv, err := getDevisByID(id)
	if err != nil {
		http.Redirect(w, r, "/devis?err=Devis+introuvable", http.StatusFound)
		return
	}
	gs := getSettings()

	isOR := dv.Statut == "accepte" || dv.Statut == "facture"
	pdf := buildDevisPDF(dv, gs, isOR)

	var prefix string
	if isOR {
		prefix = "OR"
	} else {
		prefix = "DEV"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+prefix+`-`+dv.Numero+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		log.Println("devisPDF:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
	}
}

// ─── Construction du PDF ─────────────────────────────────────────────────────

func buildDevisPDF(dv Devis, gs map[string]string, isOR bool) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 22)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pageW, pageH := pdf.GetPageSize()
	ml, mt, mr, mb := pdf.GetMargins()
	cw := pageW - ml - mr

	fill := func(c rgb) { pdf.SetFillColor(c.r, c.g, c.b) }
	draw := func(c rgb) { pdf.SetDrawColor(c.r, c.g, c.b) }
	txt := func(c rgb) { pdf.SetTextColor(c.r, c.g, c.b) }
	lw := func(v float64) { pdf.SetLineWidth(v) }

	var docLabel string
	if isOR {
		docLabel = "ORDRE DE RÉPARATION"
	} else {
		docLabel = "DEVIS"
	}

	// ══ EN-TÊTE ══════════════════════════════════════════════
	// Colonnes : gauche 42% | centre 35% | droite 23%
	leftW   := cw * 0.42
	centerW := cw * 0.35
	rightW  := cw * 0.23

	// Nom du garage (gauche, gras)
	pdf.SetFont("Arial", "B", 13)
	txt(cDark)
	pdf.SetXY(ml, mt)
	pdf.Cell(leftW, 7, tr(gs["nom_garage"]))

	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	if gs["adresse_garage"] != "" {
		pdf.SetXY(ml, mt+8)
		pdf.Cell(leftW, 4, tr(gs["adresse_garage"]))
	}
	ville := gs["ville_garage"]
	tel := gs["telephone_garage"]
	info := ville
	if tel != "" {
		if info != "" {
			info += " — " + tel
		} else {
			info = tel
		}
	}
	if info != "" {
		pdf.SetXY(ml, mt+13)
		pdf.Cell(leftW, 4, tr(info))
	}

	// Titre document (centre, gras)
	pdf.SetFont("Arial", "B", 17)
	txt(cDark)
	pdf.SetXY(ml+leftW, mt+2)
	pdf.CellFormat(centerW, 10, tr(docLabel), "", 0, "C", false, 0, "")

	// Réf + dates (droite, aligné à droite)
	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	pdf.SetXY(ml+leftW+centerW, mt)
	pdf.CellFormat(rightW, 5, tr("Réf : "+dv.Numero), "", 0, "R", false, 0, "")
	pdf.SetXY(ml+leftW+centerW, mt+6)
	pdf.CellFormat(rightW, 5, tr("Date : "+fmtDatePDF(dv.DateCreation)), "", 0, "R", false, 0, "")
	if dv.DateValidite != "" {
		pdf.SetXY(ml+leftW+centerW, mt+12)
		pdf.CellFormat(rightW, 5, tr("Valable : "+fmtDatePDF(dv.DateValidite)), "", 0, "R", false, 0, "")
	}

	// Ligne séparatrice
	pdf.SetY(mt + 22)
	draw(cGrayBdr)
	lw(0.5)
	pdf.Line(ml, pdf.GetY(), ml+cw, pdf.GetY())
	pdf.Ln(7)

	// ══ BLOC CLIENT + VÉHICULE ═══════════════════════════════
	infoY := pdf.GetY()
	hw := (cw - 5) / 2

	draw(cGrayBdr)
	lw(0.25)

	// Bloc client
	fill(cGrayBg)
	pdf.RoundedRect(ml, infoY, hw, 26, 2, "1234", "FD")
	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7)
	pdf.SetXY(ml+4, infoY+3)
	pdf.Cell(hw-8, 4, "CLIENT")
	txt(cDark)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(ml+4, infoY+8)
	pdf.Cell(hw-8, 6, tr(dv.ClientNom))
	if dv.ClientTel != "" {
		txt(cTextLt)
		pdf.SetFont("Arial", "", 9)
		pdf.SetXY(ml+4, infoY+16)
		pdf.Cell(hw-8, 4, tr("Tél : "+dv.ClientTel))
	}

	// Bloc véhicule
	vx := ml + hw + 5
	fill(cGrayBg)
	pdf.RoundedRect(vx, infoY, hw, 26, 2, "1234", "FD")
	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7)
	pdf.SetXY(vx+4, infoY+3)
	pdf.Cell(hw-8, 4, tr("VÉHICULE"))
	txt(cDark)
	pdf.SetFont("Arial", "B", 13)
	pdf.SetXY(vx+4, infoY+8)
	pdf.Cell(hw-8, 7, tr(dv.Immatriculation))
	txt(cTextLt)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(vx+4, infoY+17)
	pdf.Cell(hw-8, 4, tr(dv.VehiculeLabel))

	pdf.SetY(infoY + 34)

	// ══ DESCRIPTION ══════════════════════════════════════════
	if dv.Description != "" {
		txt(cTextLt)
		pdf.SetFont("Arial", "I", 9)
		pdf.SetX(ml)
		pdf.MultiCell(cw, 5, tr(dv.Description), "", "L", false)
		pdf.Ln(3)
	}

	// ══ TABLEAU DES LIGNES ═══════════════════════════════════
	tableY := pdf.GetY()
	col := []float64{
		cw * 0.14,
		cw * 0.33,
		cw * 0.08,
		cw * 0.13,
		cw * 0.08,
		cw * 0.12,
		cw * 0.12,
	}
	headers := []string{"Type", tr("Désignation"), tr("Qté"), "P.U. HT", "TVA", "Total HT", "TTC"}
	aligns  := []string{"L", "L", "R", "R", "R", "R", "R"}

	// En-tête tableau (noir/gris foncé)
	fill(cDark)
	txt(cWhite)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(ml, tableY)
	for i, h := range headers {
		pdf.CellFormat(col[i], 7, h, "0", 0, aligns[i], true, 0, "")
	}
	pdf.Ln(7)

	// Lignes
	odd := true
	for _, l := range dv.Lignes {
		var bg rgb
		if odd {
			bg = rgb{249, 250, 251}
		} else {
			bg = cWhite
		}
		odd = !odd

		y0 := pdf.GetY()
		fill(bg)
		txt(cDark)
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
		pdf.CellFormat(col[6], rowH, fmt.Sprintf("%.2f", l.MontantTTC), "0", 1, "R", true, 0, "")
	}

	tableEndY := pdf.GetY()
	draw(cGrayBdr)
	lw(0.25)
	pdf.Rect(ml, tableY+7, cw, tableEndY-tableY-7, "D")

	// ══ TOTAUX ════════════════════════════════════════════════
	pdf.Ln(6)
	totW := 75.0
	totX := ml + cw - totW

	// Ligne HT
	draw(cGrayBdr)
	lw(0.2)
	fill(cGrayBg)
	pdf.Rect(totX, pdf.GetY(), totW, 7, "FD")
	txt(cTextLt)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(totX+3, pdf.GetY()+1.5)
	pdf.Cell(totW*0.55, 5, tr("Total HT :"))
	txt(cDark)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(totW*0.45-3, 5, fmt.Sprintf("%.2f DZD", dv.MontantHT))
	pdf.Ln(7)

	// Ligne TVA
	fill(cGrayBg)
	pdf.Rect(totX, pdf.GetY(), totW, 7, "FD")
	txt(cTextLt)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(totX+3, pdf.GetY()+1.5)
	pdf.Cell(totW*0.55, 5, tr("TVA :"))
	txt(cDark)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(totW*0.45-3, 5, fmt.Sprintf("%.2f DZD", dv.MontantTVA))
	pdf.Ln(7)

	// Séparateur
	lineY := pdf.GetY()
	draw(cGrayBdr)
	lw(0.5)
	pdf.Line(totX, lineY, ml+cw, lineY)
	pdf.Ln(2)

	// Total TTC (fond noir)
	fill(cDark)
	ttcY := pdf.GetY()
	pdf.Rect(totX, ttcY, totW, 11, "F")
	txt(cWhite)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(totX+3, ttcY+2)
	pdf.Cell(totW*0.5, 7, "TOTAL TTC")
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(totW*0.5-3, 7, fmt.Sprintf("%.2f DZD", dv.MontantTTC))
	pdf.Ln(18)

	// ══ ZONE SIGNATURE (OR uniquement) ═══════════════════════
	if isOR {
		sigY := pdf.GetY()
		draw(cGrayBdr)
		lw(0.25)
		fill(cGrayBg)
		pdf.Rect(ml, sigY, cw, 38, "FD")

		txt(cDark)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetXY(ml+4, sigY+4)
		pdf.Cell(cw-8, 5, tr("AUTORISATION DE RÉPARATION — BON POUR ACCORD"))

		pdf.SetFont("Arial", "", 8)
		pdf.SetXY(ml+4, sigY+11)
		pdf.MultiCell(cw*0.55, 4.5,
			tr("Je soussigné(e) "+dv.ClientNom+
				" autorise le garage «"+gs["nom_garage"]+"» à effectuer les travaux"+
				" ci-dessus pour un montant estimé de "+fmt.Sprintf("%.2f DZD TTC.", dv.MontantTTC)+
				"\n\nLu et approuvé — Date : _______________"),
			"", "L", false)

		sigBoxX := ml + cw*0.64
		txt(cTextLt)
		pdf.SetFont("Arial", "", 8)
		pdf.SetXY(sigBoxX, sigY+11)
		pdf.Cell(cw*0.33, 4, "Signature du client :")
		draw(cGrayBdr)
		lw(0.3)
		pdf.Rect(sigBoxX, sigY+17, cw*0.33, 18, "D")
	}

	// ══ MENTIONS LÉGALES + PIED DE PAGE ══════════════════════════
	footY := pageH - mb - 8
	_ = mr

	if m := gs["mentions_legales"]; m != "" {
		txt(cTextLt)
		pdf.SetFont("Arial", "I", 7.5)
		pdf.SetXY(ml, footY-12)
		pdf.MultiCell(cw, 3.8, tr(m), "", "C", false)
	}

	draw(cGrayBdr)
	lw(0.4)
	pdf.Line(ml, footY-4, pageW-mr, footY-4)
	txt(cTextLt)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetXY(ml, footY)
	pdf.CellFormat(cw, 5,
		tr(gs["nom_garage"]+" — "+gs["adresse_garage"]+" — "+gs["telephone_garage"]),
		"", 0, "C", false, 0, "")

	return pdf
}
