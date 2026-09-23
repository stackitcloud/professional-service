// Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chart

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
)

// presignDuration is how long a chart URL stays valid (7 days).
const presignDuration = 7 * 24 * time.Hour

// Uploader uploads rendered charts to STACKIT Object Storage (S3-
// compatible) and returns presigned GET URLs. Objects are private;
// only the presigned URL grants access.
type Uploader struct {
	client *s3.Client
	bucket string
}

// NewUploader builds an S3 client bound to the STACKIT Object Storage
// endpoint (custom endpoint + path-style, required for STACKIT S3) using
// the configured static credentials. All five S3 fields must be present
// (validated upstream by config).
func NewUploader(ctx context.Context, s config.S3Config) (*Uploader, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(s.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s.AccessKey, s.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("loading S3 SDK config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s.Endpoint)
		o.UsePathStyle = true
	})
	return &Uploader{client: client, bucket: s.Bucket}, nil
}

// UploadAndPresign uploads the PNG privately (no ACL) and returns a
// presigned GET URL valid for presignDuration.
func (u *Uploader) UploadAndPresign(ctx context.Context, key string, png *bytes.Buffer) (string, error) {
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(png.Bytes()),
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		return "", fmt.Errorf("uploading chart object %s: %w", key, err)
	}

	presignClient := s3.NewPresignClient(u.client)
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(presignDuration))
	if err != nil {
		return "", fmt.Errorf("presigning chart URL: %w", err)
	}
	return presignedReq.URL, nil
}
