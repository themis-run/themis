package themisclient

type Config struct {
	ServerName    string
	ServerAddress string

	Servers map[string]string

	LoadBalancerName string

	RetryNum int
}
