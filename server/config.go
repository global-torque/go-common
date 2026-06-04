package server

// Config is struct to configure HTTP server
type Config struct {
	Host                     string `required:"true"`
	Port                     string `required:"true"`
	CORSAllowedOrigins       string `split_words:"true"`
	ReadTimeoutSeconds       int    `default:"15" split_words:"true"`
	ReadHeaderTimeoutSeconds int    `default:"5" split_words:"true"`
	WriteTimeoutSeconds      int    `default:"30" split_words:"true"`
	IdleTimeoutSeconds       int    `default:"120" split_words:"true"`
	StartupGraceMilliseconds int    `default:"100" split_words:"true"`
}
