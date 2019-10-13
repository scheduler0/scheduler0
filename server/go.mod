module cron-server/server

go 1.13

require (
	github.com/go-pg/pg v8.0.6+incompatible
	github.com/go-redis/redis v6.15.5+incompatible
	github.com/gorilla/mux v1.7.3
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/robfig/cron v1.2.0
	github.com/segmentio/ksuid v1.0.2
	github.com/stretchr/testify v1.4.0
	github.com/unrolled/secure v1.0.4
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/tools v0.0.0-20191012152004-8de300cfc20a // indirect
	mellium.im/sasl v0.2.1 // indirect
)
