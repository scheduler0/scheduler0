package models

import "time"

type CredentialModel struct {
	TableName struct{} `sql:"credentials"`

	ID                            int64     `json:"id" sql:",pk:notnull"`
	UUID                          string    `json:"uuid" sql:",unique,type:uuid,default:gen_random_uuid()"`
	Archived                      bool      `json:"archived" sql:",notnull"`
	Platform                      string    `json:"platform" sql:",notnull"`
	ApiKey                        string    `json:"api_key" sql:",null"`
	ApiSecret                     string    `json:"api_secret" sql:",null"`
	IPRestriction                 string    `json:"ip_restriction" sql:",null"`
	HTTPReferrerRestriction       string    `json:"http_referrer_restriction" sql:",null"`
	IOSBundleIDRestriction        string    `json:"ios_bundle_id_restriction" sql:",null"`
	AndroidPackageNameRestriction string    `json:"android_package_name_restriction" sql:",null"`
	DateCreated                   time.Time `json:"date_created" sql:",notnull,default:now()"`
}
