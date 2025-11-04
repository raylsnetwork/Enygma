package config


type Config struct {
    Port     string
    EnygmaPk  string
    EnygmaVk  string
	WithdrawPk1 string
	WithdrawVk1 string
    WithdrawPk2 string
	WithdrawVk2 string
    WithdrawPk3 string
	WithdrawVk3 string
    WithdrawPk4 string
	WithdrawVk4 string
    WithdrawPk5 string
	WithdrawVk5 string
    WithdrawPk6 string
	WithdrawVk6 string
	DepositPk string
	DepositVk string

}

func Load() *Config {
    return &Config{
        Port:    	   "8080",
        EnygmaPk:      "./keys/EnygmaPk.key",
        EnygmaVk: 	   "./keys/EnygmaVk.key",
        WithdrawPk1:    "./keys/zkdvp/WithdrawPk1.key",
        WithdrawVk1:    "./keys/zkdvp/WithdrawVk1.key",
        WithdrawPk2:    "./keys/zkdvp/WithdrawPk2.key",
        WithdrawVk2:    "./keys/zkdvp/WithdrawVk2.key",
        WithdrawPk3:    "./keys/zkdvp/WithdrawPk3.key",
        WithdrawVk3:    "./keys/zkdvp/WithdrawVk3.key",
        WithdrawPk4:    "./keys/zkdvp/WithdrawPk4.key",
        WithdrawVk4:    "./keys/zkdvp/WithdrawVk4.key",
        WithdrawPk5:    "./keys/zkdvp/WithdrawPk5.key",
        WithdrawVk5:    "./keys/zkdvp/WithdrawVk5.key",
        WithdrawPk6:    "./keys/zkdvp/WithdrawPk6.key",
        WithdrawVk6:    "./keys/zkdvp/WithdrawVk6.key",

        DepositPk:     "./keys/DepositPk.key",
        DepositVk:     "./keys/DepositKVk.key",
    }
}

