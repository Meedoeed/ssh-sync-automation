package domain

type Server struct {
	ID         string
	Name       string
	Host       string
	Port       int
	Username   string
	AuthType   string
	Password   *string
	PrivateKey *string
}
