package cli

import (
	"fmt"
	"github.com/urfave/cli"
	"github.com/way365/bazo-miner/crypto"
)

func GetGenerateWalletCommand() cli.Command {
	return cli.Command{
		Name:  "generate-wallet",
		Usage: "generate a new pair of wallet keys",
		Action: func(c *cli.Context) error {
			filename := c.String("file")
			privKey, err := crypto.ExtractECDSAKeyFromFile(filename)

			fmt.Printf("Wallet generated successfully.\n")
			fmt.Printf("PubKeyX: %x\n", privKey.PublicKey.X)
			fmt.Printf("PubKeyY: %x\n", privKey.PublicKey.Y)
			fmt.Printf("PrivKey: %x\n", privKey.D)

			return err
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "file",
				Usage: "the new key's `FILE` name",
			},
		},
	}
}
