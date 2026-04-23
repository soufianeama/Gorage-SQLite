package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func stockListHandler(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	categorie := r.URL.Query().Get("categorie")

	pieces, _ := getPieces(search, categorie)
	categories, _ := getCategories()

	type StockData struct {
		Pieces     []Piece
		Categories []string
		Search     string
		Categorie  string
	}

	renderPage(w, "stock_list", PageData{
		Title:      "Stock pièces",
		ActivePage: "stock",
		Session:    getSession(r),
		Data: StockData{
			Pieces:     pieces,
			Categories: categories,
			Search:     search,
			Categorie:  categorie,
		},
		Success: r.URL.Query().Get("ok"),
		Error:   r.URL.Query().Get("err"),
	})
}

func stockCreateHandler(w http.ResponseWriter, r *http.Request) {
	nom := strings.TrimSpace(r.FormValue("nom"))
	if nom == "" {
		http.Redirect(w, r, "/stock?err=Le+nom+est+obligatoire", http.StatusFound)
		return
	}
	ref := strings.TrimSpace(r.FormValue("reference"))
	cat := strings.TrimSpace(r.FormValue("categorie"))
	fourn := strings.TrimSpace(r.FormValue("fournisseur"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	qte, _ := strconv.ParseInt(r.FormValue("quantite_stock"), 10, 64)
	seuil, _ := strconv.ParseInt(r.FormValue("seuil_alerte"), 10, 64)
	prixAchat, _ := strconv.ParseFloat(r.FormValue("prix_achat"), 64)
	prixVente, _ := strconv.ParseFloat(r.FormValue("prix_vente"), 64)
	if seuil <= 0 {
		seuil = 5
	}

	_, err := createPiece(ref, nom, cat, qte, seuil, prixAchat, prixVente, fourn, notes)
	if err != nil {
		log.Println("createPiece:", err)
		http.Redirect(w, r, "/stock?err=Erreur+lors+de+l'ajout+de+la+pièce", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/stock?ok=Pièce+ajoutée", http.StatusFound)
}

func stockEditGetHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	p, err := getPieceByID(id)
	if err != nil {
		http.Redirect(w, r, "/stock?err=Pièce+introuvable", http.StatusFound)
		return
	}
	categories, _ := getCategories()

	type EditData struct {
		Piece      Piece
		Categories []string
	}

	renderPage(w, "stock_form", PageData{
		Title:      "Modifier pièce",
		ActivePage: "stock",
		Session:    getSession(r),
		Data:       EditData{Piece: p, Categories: categories},
	})
}

func stockEditPostHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	nom := strings.TrimSpace(r.FormValue("nom"))
	if nom == "" {
		http.Redirect(w, r, fmt.Sprintf("/stock/edit?id=%d&err=Le+nom+est+obligatoire", id), http.StatusFound)
		return
	}
	ref := strings.TrimSpace(r.FormValue("reference"))
	cat := strings.TrimSpace(r.FormValue("categorie"))
	fourn := strings.TrimSpace(r.FormValue("fournisseur"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	qte, _ := strconv.ParseInt(r.FormValue("quantite_stock"), 10, 64)
	seuil, _ := strconv.ParseInt(r.FormValue("seuil_alerte"), 10, 64)
	prixAchat, _ := strconv.ParseFloat(r.FormValue("prix_achat"), 64)
	prixVente, _ := strconv.ParseFloat(r.FormValue("prix_vente"), 64)
	if seuil <= 0 {
		seuil = 5
	}

	if err := updatePiece(id, ref, nom, cat, qte, seuil, prixAchat, prixVente, fourn, notes); err != nil {
		log.Println("updatePiece:", err)
		http.Redirect(w, r, "/stock?err=Erreur+lors+de+la+mise+à+jour", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/stock?ok=Pièce+mise+à+jour", http.StatusFound)
}

func stockAjusterHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	delta, _ := strconv.ParseInt(r.FormValue("delta"), 10, 64)
	if delta == 0 {
		http.Redirect(w, r, "/stock", http.StatusFound)
		return
	}
	if err := ajusterStock(id, delta); err != nil {
		log.Println("ajusterStock:", err)
		http.Redirect(w, r, "/stock?err=Erreur+lors+de+l'ajustement+du+stock", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/stock?ok=Stock+mis+à+jour", http.StatusFound)
}

func stockDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	deletePiece(id) //nolint:errcheck
	http.Redirect(w, r, "/stock?ok=Pièce+supprimée", http.StatusFound)
}
