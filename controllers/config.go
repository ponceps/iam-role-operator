package controllers

import (
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

var awsRegion = os.Getenv("AWS_REGION")
var awsAccountID = os.Getenv("AWS_ACCOUNT_ID")
var openIDIssuer = strings.TrimPrefix(os.Getenv("OPENID_ISSUER_URL"), "https://")

var awsSession = session.Must(session.NewSession(&aws.Config{
	Region: aws.String(awsRegion)},
))
