package themisclient

type Config struct {
	ServerName    string
	ServerAddress string

	LoadBalancerName string
	CodecType        string

	RetryNum int
}
