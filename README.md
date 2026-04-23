# L'Atelier Go

Logiciel de gestion de garage automobile, développé en Go. Application web locale, sans dépendance externe au runtime — un seul binaire suffit.

---

## Fonctionnalités

### Clients & Véhicules
- Fiche client (nom, téléphone, email, adresse)
- Fiche véhicule (marque, modèle, année, immatriculation, VIN, kilométrage)
- Historique des interventions, devis et rendez-vous par véhicule

### Interventions
- Kanban 4 colonnes : En cours / Attente pièces / Terminé / Livré
- Saisie diagnostic, travaux effectués, pièces utilisées
- Impression fiche d'accueil véhicule (PDF)
- Génération de facture interne depuis une intervention

### Devis / Ordres de Réparation
- Numérotation automatique `DEV-YYYY-XXXX`
- Lignes main d'œuvre, pièces, fournitures avec TVA configurable
- Statuts : Brouillon → Envoyé → Accepté (OR) → Refusé → Facturé
- Impression PDF devis ou OR
- Envoi par email avec PDF en pièce jointe

### Facturation officielle
- Numérotation `FAC-YYYY-XXXX` immuable (loi anti-fraude)
- Emise depuis un devis accepté ou directement
- Statuts : Émise → Payée / Annulée
- Émission d'avoirs `AV-YYYY-XXXX` avec annulation automatique de la facture d'origine
- Impression PDF conforme avec identifiants fiscaux (NIF, NIS, RC, AI)
- Envoi par email avec PDF en pièce jointe

### Rendez-vous
- Vue liste et vue calendrier hebdomadaire
- Statuts : Planifié / Confirmé / Annulé

### Stock pièces
- Gestion par référence, catégorie, fournisseur
- Seuil d'alerte stock bas visible sur le tableau de bord
- Ajustement manuel du stock

### Statistiques
- Graphique interventions par mois (12 derniers mois)
- Top 10 pièces les plus utilisées

### Multi-utilisateurs
- Rôles : **Admin** (accès complet) / **Mécanicien** (Dashboard, RDV, Clients, Véhicules, Interventions)
- Authentification bcrypt, sessions sécurisées (HttpOnly, SameSite=Strict)
- Protection CSRF sur tous les formulaires
- Rate limiting sur la page de connexion (5 tentatives → blocage 15 min)

### Sauvegarde
- Téléchargement local de la base de données (SQLite VACUUM)
- Upload vers un stockage cloud compatible S3 (Cloudflare R2, Backblaze B2, AWS S3)

---

## Stack technique

| Composant | Technologie |
|-----------|-------------|
| Langage | Go 1.25 |
| Base de données | SQLite (`modernc.org/sqlite` — sans cgo) |
| Génération PDF | `github.com/go-pdf/fpdf` |
| Envoi email | `gopkg.in/mail.v2` (SMTP) |
| Hachage mots de passe | `golang.org/x/crypto/bcrypt` |
| Stockage cloud | `github.com/aws/aws-sdk-go-v2/service/s3` |
| Frontend | HTML/CSS pur (pas de framework) |
| Templates | `html/template` (Go stdlib) |

---

## Installation

### Prérequis

- Go 1.25+

### Cloner et lancer

```bash
git clone https://github.com/soufianeama/Gorage-SQLite.git
cd gorage
go run .
```

L'application s'ouvre automatiquement sur `http://127.0.0.1:8585`.

Au premier lancement, une page de configuration vous demande de créer le compte administrateur.

### Compiler en binaire

```bash
go build -o gorage .
./gorage
```

---

## Configuration

Tous les paramètres sont accessibles dans **Paramètres** (menu sidebar, admin uniquement).

### Informations garage
Nom, adresse, téléphone, identifiants fiscaux (NIF, NIS, RC, AI).

### Email SMTP

| Champ | Description |
|-------|-------------|
| Serveur SMTP | ex : `smtp.gmail.com` |
| Port | `587` (TLS) ou `465` (SSL) |
| Utilisateur | Adresse email expéditeur |
| Mot de passe | Pour Gmail : utiliser un App Password |
| Nom expéditeur | Nom affiché dans les emails |

### Sauvegarde cloud (S3 / R2 / B2)

| Champ | Cloudflare R2 |
|-------|---------------|
| Endpoint | `https://<account_id>.r2.cloudflarestorage.com` |
| Bucket | Nom du bucket R2 |
| Région | `auto` |
| Access Key ID | Depuis Manage R2 API Tokens |
| Secret Access Key | Depuis Manage R2 API Tokens |

---

## Structure des fichiers

```
gorage/
├── main.go                 # Routage, templates, point d'entrée
├── auth.go                 # Sessions, bcrypt, CSRF, rate limiting
├── database.go             # Schéma SQLite, migrations, requêtes
├── handlers.go             # Handlers HTTP (login, clients, véhicules...)
├── devis.go                # Devis & ordres de réparation
├── factures.go             # Facturation officielle & avoirs
├── rdv.go                  # Rendez-vous & calendrier
├── stock.go                # Gestion du stock pièces
├── email.go                # Envoi email SMTP
├── backup_cloud.go         # Sauvegarde vers stockage S3
├── pdf.go                  # PDF facture intervention
├── pdf_devis.go            # PDF devis / OR
├── pdf_facture_legale.go   # PDF facture officielle & avoir
├── pdf_fiche_accueil.go    # PDF fiche d'accueil véhicule
├── static/
│   └── app.css             # CSS custom (sans framework)
└── templates/
    ├── base.html           # Layout principal avec sidebar
    ├── dashboard.html
    ├── clients.html
    ├── vehicules.html
    ├── interventions.html
    ├── devis_*.html
    ├── facture_*.html
    ├── rdv_*.html
    ├── stock_*.html
    ├── stats.html
    ├── settings.html
    └── ...
```

---

## Licence

GPL
