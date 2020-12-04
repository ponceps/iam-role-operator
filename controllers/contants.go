package controllers

import (
	"os"
	"strings"
)

var awsRegion = os.Getenv("AWS_REGION")
var awsAccountID = os.Getenv("AWS_ACCOUNT_ID")
var openIDIssuer = strings.TrimPrefix(os.Getenv("OPENID_ISSUER_URL"), "https://")
