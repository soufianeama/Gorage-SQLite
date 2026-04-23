package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func newS3Client(gs map[string]string) (*s3.Client, string, error) {
	endpoint  := gs["s3_endpoint"]
	bucket    := gs["s3_bucket"]
	accessKey := gs["s3_access_key"]
	secretKey := gs["s3_secret_key"]
	region    := gs["s3_region"]

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, "", fmt.Errorf("configuration cloud incomplète — vérifiez les paramètres Sauvegarde cloud")
	}
	if region == "" {
		region = "auto"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, "", err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return client, bucket, nil
}

func uploadBackupToCloud() error {
	gs := getSettings()
	client, bucket, err := newS3Client(gs)
	if err != nil {
		return err
	}

	// Générer le backup en mémoire via VACUUM INTO fichier tmp
	tmp := fmt.Sprintf("/tmp/gorage-cloud-%d.db", time.Now().UnixNano())
	if _, err := db.Exec("VACUUM INTO ?", tmp); err != nil {
		return fmt.Errorf("erreur génération backup : %w", err)
	}
	defer os.Remove(tmp)

	data, err := os.ReadFile(tmp)
	if err != nil {
		return fmt.Errorf("erreur lecture backup : %w", err)
	}

	key := fmt.Sprintf("gorage-backup-%s.db", time.Now().Format("2006-01-02-150405"))
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("erreur upload : %w", err)
	}
	return nil
}

func cloudBackupHandler(w http.ResponseWriter, r *http.Request) {
	if err := uploadBackupToCloud(); err != nil {
		log.Println("cloudBackup:", err)
		http.Redirect(w, r, "/settings?err="+url.QueryEscape(err.Error())+"#donnees", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/settings?ok=Sauvegarde+cloud+effectuée#donnees", http.StatusFound)
}
