package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
)

// ─── Handler HTTP ─────────────────────────────────────────────────────────────

func ficheAccueilHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	inv, err := getInterventionByID(id)
	if err != nil {
		http.Redirect(w, r, "/interventions?err=Intervention+introuvable", http.StatusFound)
		return
	}
	veh, _ := getVehiculeByID(inv.VehiculeID)
	gs := getSettings()
	pdf := buildFicheAccueilPDF(inv, veh, gs)

	filename := fmt.Sprintf("accueil-%04d.pdf", id)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	if err := pdf.Output(w); err != nil {
		log.Println("ficheAccueilPDF:", err)
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
	}
}

// ─── Construction PDF ─────────────────────────────────────────────────────────

func buildFicheAccueilPDF(inv Intervention, veh Vehicule, gs map[string]string) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 20)
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

	leftW  := cw * 0.52
	rightW := cw * 0.44

	// Gauche : titre
	txt(cDark)
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(ml, mt)
	pdf.Cell(leftW, 10, tr("FICHE D'ACCUEIL VÉHICULE"))

	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	pdf.SetXY(ml, mt+12)
	pdf.Cell(leftW, 5, tr(fmt.Sprintf("Intervention N° %04d", inv.ID)))
	pdf.SetXY(ml, mt+18)
	pdf.Cell(leftW, 5, tr("Date d'entrée : "+fmtDatePDF(inv.DateEntree)))
	pdf.SetXY(ml, mt+24)
	pdf.Cell(leftW, 5, tr("Imprimé le : "+time.Now().Format("02/01/2006 15:04")))

	// Droite : bloc garage
	bx := ml + leftW + cw*0.04
	bH := 35.0
	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)
	pdf.Rect(bx, mt-1, rightW, bH, "FD")

	txt(cDark)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(bx+4, mt+3)
	pdf.Cell(rightW-8, 6, tr(gs["nom_garage"]))

	pdf.SetFont("Arial", "", 8)
	txt(cTextLt)
	pdf.SetXY(bx+4, mt+11)
	addr := gs["adresse_garage"]
	if gs["ville_garage"] != "" {
		addr += "\n" + gs["ville_garage"]
	}
	if gs["telephone_garage"] != "" {
		addr += "\n" + gs["telephone_garage"]
	}
	pdf.MultiCell(rightW-8, 4, tr(addr), "", "L", false)

	pdf.SetY(mt + bH + 5)

	// ════════════════════════════════════════════════════
	// SECTION 2 — CLIENT + VÉHICULE
	// ════════════════════════════════════════════════════

	infoY := pdf.GetY()
	hw    := (cw - 6) / 2
	infoH := 36.0

	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)

	// Bloc client
	pdf.Rect(ml, infoY, hw, infoH, "FD")
	pdf.SetFont("Arial", "B", 7.5)
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
		pdf.SetXY(ml+4, infoY+17)
		pdf.Cell(hw-8, 4, tr("Tél : "+inv.ClientTel))
	}
	if inv.ClientAdresse != "" {
		pdf.SetXY(ml+4, infoY+22)
		pdf.MultiCell(hw-8, 4, tr(inv.ClientAdresse), "", "L", false)
	}

	// Bloc véhicule
	vx := ml + hw + 6
	pdf.Rect(vx, infoY, hw, infoH, "FD")
	pdf.SetFont("Arial", "B", 7.5)
	txt(cTextLt)
	pdf.SetXY(vx+4, infoY+3)
	pdf.Cell(hw-8, 4, tr("VÉHICULE"))
	pdf.SetFont("Arial", "B", 13)
	txt(cDark)
	pdf.SetXY(vx+4, infoY+9)
	pdf.Cell(hw-8, 7, tr(inv.Immatriculation))
	pdf.SetFont("Arial", "", 9)
	txt(cTextLt)
	pdf.SetXY(vx+4, infoY+17)
	pdf.Cell(hw-8, 4, tr(inv.Marque+" "+inv.Modele))
	if veh.Annee > 0 {
		pdf.SetXY(vx+4, infoY+22)
		pdf.Cell(hw/2, 4, tr(fmt.Sprintf("Année : %d", veh.Annee)))
	}
	if veh.Kilometrage > 0 {
		pdf.SetXY(vx+4+hw/2, infoY+22)
		pdf.Cell(hw/2-4, 4, tr(fmt.Sprintf("Km : %d", veh.Kilometrage)))
	}
	if veh.VIN != "" {
		pdf.SetXY(vx+4, infoY+27)
		pdf.Cell(hw-8, 4, tr("VIN : "+veh.VIN))
	}

	pdf.SetY(infoY + infoH + 6)

	// ════════════════════════════════════════════════════
	// SECTION 3 — MOTIF DE L'INTERVENTION
	// ════════════════════════════════════════════════════

	motifY := pdf.GetY()
	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(ml, motifY)
	pdf.Cell(cw, 4, tr("PROBLÈME SIGNALÉ PAR LE CLIENT"))
	pdf.Ln(5)

	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)
	boxH := 16.0
	pdf.Rect(ml, pdf.GetY(), cw, boxH, "FD")

	if inv.Description != "" {
		txt(cDark)
		pdf.SetFont("Arial", "", 9)
		pdf.SetXY(ml+3, pdf.GetY()+2)
		pdf.MultiCell(cw-6, 4.5, tr(inv.Description), "", "L", false)
	}
	pdf.SetY(motifY + 5 + boxH + 6)

	// ════════════════════════════════════════════════════
	// SECTION 4 — GRILLE D'INSPECTION VISUELLE
	// ════════════════════════════════════════════════════

	gridY := pdf.GetY()
	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(ml, gridY)
	pdf.Cell(cw, 4, tr("ÉTAT DU VÉHICULE À L'ENTRÉE"))
	pdf.Ln(5)

	gridY = pdf.GetY()
	colItem := cw * 0.48
	colBon  := cw * 0.14
	colDef  := cw * 0.14
	colRem  := cw * 0.24
	rowH    := 7.0

	// En-tête tableau
	fill(cDark)
	txt(cWhite)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(ml, gridY)
	pdf.CellFormat(colItem, rowH, tr("  Élément"), "0", 0, "L", true, 0, "")
	pdf.CellFormat(colBon,  rowH, tr("Bon"), "0", 0, "C", true, 0, "")
	pdf.CellFormat(colDef,  rowH, tr("Défaut"), "0", 0, "C", true, 0, "")
	pdf.CellFormat(colRem,  rowH, tr("Remarque"), "0", 1, "L", true, 0, "")

	items := []string{
		tr("Carrosserie — Avant"),
		tr("Carrosserie — Arrière"),
		tr("Flanc gauche"),
		tr("Flanc droit"),
		tr("Pare-brise"),
		tr("Lunette arrière"),
		tr("Vitres latérales"),
		tr("Jantes / Pneus"),
		tr("Intérieur / Sièges"),
		tr("Tableau de bord"),
		tr("Éclairages"),
		tr("Accessoires / Rétroviseurs"),
	}

	draw(cGrayBdr)
	lw(0.25)
	for i, item := range items {
		y0 := pdf.GetY()
		if i%2 == 0 {
			fill(cGrayBg)
		} else {
			fill(cWhite)
		}
		txt(cDark)
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetXY(ml, y0)
		pdf.CellFormat(colItem, rowH, "  "+item, "0", 0, "L", true, 0, "")

		// Cases à cocher (carrés vides)
		fill(cWhite)
		draw(cGrayBdr)
		lw(0.4)
		cx1 := ml + colItem + (colBon-4)/2
		cx2 := ml + colItem + colBon + (colDef-4)/2
		cy  := y0 + (rowH-3.5)/2
		// Case Bon
		pdf.SetFillColor(255, 255, 255)
		pdf.Rect(cx1, cy, 3.5, 3.5, "FD")
		// Case Défaut
		pdf.Rect(cx2, cy, 3.5, 3.5, "FD")

		// Remplissage des colonnes avec la couleur de fond
		if i%2 == 0 {
			fill(cGrayBg)
		} else {
			fill(cWhite)
		}
		lw(0)
		pdf.Rect(ml+colItem, y0, colBon, rowH, "F")
		pdf.Rect(ml+colItem+colBon, y0, colDef, rowH, "F")

		// Re-dessiner les cases
		fill(cWhite)
		draw(cGrayBdr)
		lw(0.4)
		pdf.Rect(cx1, cy, 3.5, 3.5, "FD")
		pdf.Rect(cx2, cy, 3.5, 3.5, "FD")

		// Colonne Remarque (ligne de saisie)
		rxStart := ml + colItem + colBon + colDef
		lw(0.3)
		draw(cGrayBdr)
		if i%2 == 0 {
			fill(cGrayBg)
		} else {
			fill(cWhite)
		}
		pdf.Rect(rxStart, y0, colRem, rowH, "FD")

		pdf.SetY(y0 + rowH)
	}

	// Bordure extérieure du tableau
	draw(cGrayBdr)
	lw(0.4)
	tableEndY := pdf.GetY()
	pdf.Rect(ml, gridY+rowH, cw, tableEndY-gridY-rowH, "D")

	pdf.Ln(6)

	// ════════════════════════════════════════════════════
	// SECTION 5 — OBSERVATIONS
	// ════════════════════════════════════════════════════

	obsY := pdf.GetY()
	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(ml, obsY)
	pdf.Cell(cw, 4, tr("OBSERVATIONS / REMARQUES COMPLÉMENTAIRES"))
	pdf.Ln(5)

	fill(cGrayBg)
	draw(cGrayBdr)
	lw(0.3)
	pdf.Rect(ml, pdf.GetY(), cw, 18, "FD")
	// Lignes de saisie dans la zone observations
	lw(0.2)
	draw(cGrayBdr)
	for i := 0; i < 3; i++ {
		lineY := pdf.GetY() + 5.0 + float64(i)*5.5
		pdf.Line(ml+3, lineY, ml+cw-3, lineY)
	}
	pdf.Ln(24)

	// ════════════════════════════════════════════════════
	// SECTION 6 — DATES + SIGNATURES
	// ════════════════════════════════════════════════════

	sigY := pageH - mb - 38
	if pdf.GetY() > sigY-5 {
		sigY = pdf.GetY() + 3
	}

	// Dates
	txt(cTextLt)
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetXY(ml, sigY)
	pdf.Cell(cw/2, 5, tr("Date d'entrée : "+fmtDatePDF(inv.DateEntree)))
	pdf.SetXY(ml+cw/2, sigY)
	pdf.Cell(cw/2, 5, tr("Date de restitution estimée : _____ / _____ / _________"))

	pdf.Ln(10)

	// Deux colonnes de signature
	sigBoxW := (cw - 10) / 2
	sigBoxH := 22.0
	s1x     := ml
	s2x     := ml + sigBoxW + 10

	draw(cGrayBdr)
	lw(0.3)
	fill(cGrayBg)

	pdf.SetY(sigY + 8)
	sy := pdf.GetY()

	pdf.Rect(s1x, sy, sigBoxW, sigBoxH, "FD")
	pdf.Rect(s2x, sy, sigBoxW, sigBoxH, "FD")

	txt(cTextLt)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(s1x+4, sy+3)
	pdf.Cell(sigBoxW-8, 4, tr("SIGNATURE DU CLIENT"))
	pdf.SetXY(s2x+4, sy+3)
	pdf.Cell(sigBoxW-8, 4, tr("SIGNATURE DU RÉCEPTIONNISTE"))

	txt(cTextLt)
	pdf.SetFont("Arial", "I", 7.5)
	pdf.SetXY(s1x+4, sy+sigBoxH-5)
	pdf.Cell(sigBoxW-8, 4, tr("Lu et approuvé"))

	// Pied de page
	footY := pageH - mb - 8
	draw(cGrayBdr)
	lw(0.4)
	pdf.Line(ml, footY, ml+cw, footY)
	txt(cTextLt)
	pdf.SetFont("Arial", "I", 7)
	pdf.SetXY(ml, footY+2)
	pdf.CellFormat(cw, 4,
		tr(gs["nom_garage"]+" — "+gs["adresse_garage"]+" — "+gs["telephone_garage"]),
		"", 0, "C", false, 0, "")

	_ = mr
	return pdf
}
