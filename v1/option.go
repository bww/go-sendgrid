package sendgrid

type Option func(Config) Config

type Config struct {
	Endpoint        string
	OverrideAddress string
	DefaultSender   Address
	Verbose         bool
}

func Endpoint(base string) Option {
	return func(c Config) Config {
		c.Endpoint = base
		return c
	}
}

func DefaultSender(sender Address) Option {
	return func(c Config) Config {
		c.DefaultSender = sender
		return c
	}
}

func OverrideAddress(override string) Option {
	return func(c Config) Config {
		c.OverrideAddress = override
		return c
	}
}

func Verbose(on bool) Option {
	return func(c Config) Config {
		c.Verbose = on
		return c
	}
}
