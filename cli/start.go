package cli

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/way365/bazo-miner/crypto"
	"github.com/way365/bazo-miner/miner"
	"github.com/way365/bazo-miner/p2p"
	"github.com/way365/bazo-miner/storage"
	"log"
)

type startArgs struct {
	dbname               string
	myNodeAddress        string
	bootstrapNodeAddress string
	walletFile           string
	multisigFile         string
	commitmentFile       string
	rootWalletFile       string
	rootCommitmentFile   string
	rootChParamsFile     string
}

func GetStartCommand(logger *log.Logger) cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "start the miner",
		Action: func(c *cli.Context) error {
			args := &startArgs{
				dbname:               c.String("database"),
				myNodeAddress:        c.String("address"),
				bootstrapNodeAddress: c.String("bootstrap"),
				walletFile:           c.String("wallet"),
				multisigFile:         c.String("multisig"),
				commitmentFile:       c.String("commitment"),
				rootWalletFile:       c.String("rootwallet"),
				rootCommitmentFile:   c.String("rootcommitment"),
				rootChParamsFile:     c.String("root-chparams"),
			}

			if !c.IsSet("bootstrap") {
				args.bootstrapNodeAddress = args.myNodeAddress
			}

			err := args.ValidateInput()
			if err != nil {
				return err
			}

			fmt.Println(args.String())

			if c.Bool("confirm") {
				fmt.Scanf("\n")
			}

			return Start(args, logger)
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "database, d",
				Usage: "load database of the disk-based key/value store from `FILE`",
				Value: "store.db",
			},
			cli.StringFlag{
				Name:  "address, a",
				Usage: "start node at `IP:PORT`",
				Value: "localhost:8000",
			},
			cli.StringFlag{
				Name:  "bootstrap, b",
				Usage: "connect to bootstrap node at `IP:PORT`",
				Value: "localhost:8000",
			},
			cli.StringFlag{
				Name:  "wallet, w",
				Usage: "load validator's public key from `FILE`",
				Value: "wallet.txt",
			},
			cli.StringFlag{
				Name:  "multisig, m",
				Usage: "load multi-signature server’s public key from `FILE`",
			},
			cli.StringFlag{
				Name:  "commitment, c",
				Usage: "load validator's RSA public-private key from `FILE`",
				Value: "commitment.txt",
			},
			cli.StringFlag{
				Name:  "rootwallet",
				Usage: "load root's public key from `FILE`",
				Value: "wallet.txt",
			},
			cli.StringFlag{
				Name:  "rootcommitment",
				Usage: "load root's RSA public-private key from `FILE`",
				Value: "commitment.txt",
			},
			cli.StringFlag{
				Name:  "root-chparams, ch",
				Usage: "load the root chameleon hash parameters from `FILE`",
			},
			cli.BoolFlag{
				Name:  "confirm",
				Usage: "user must press enter before starting the miner",
			},
		},
	}
}

func Start(args *startArgs, logger *log.Logger) error {
	storage.Init(args.dbname, args.bootstrapNodeAddress)
	p2p.Init(args.myNodeAddress)

	validatorPubKey, err := crypto.GetOrCreateECDSAPublicKeyFromFile(args.walletFile)
	if err != nil {
		logger.Printf("%v\n", err)
		return err
	}

	rootPrivKey, err := crypto.ExtractECDSAKeyFromFile(args.rootWalletFile)
	if err != nil {
		logger.Printf("%v\n", err)
		return err
	}

	var multisigPubKey *ecdsa.PublicKey
	if len(args.multisigFile) > 0 {
		multisigPubKey, err = crypto.GetOrCreateECDSAPublicKeyFromFile(args.multisigFile)
		if err != nil {
			logger.Printf("%v\n", err)
			return err
		}
	} else {
		multisigPubKey = &rootPrivKey.PublicKey
	}

	commPrivKey, err := crypto.ExtractRSAKeyFromFile(args.commitmentFile)
	if err != nil {
		logger.Printf("%v\n", err)
		return err
	}

	rootCommPrivKey, err := crypto.ExtractRSAKeyFromFile(args.rootCommitmentFile)
	if err != nil {
		logger.Printf("%v\n", err)
		return err
	}

	rootChParams, err := crypto.GetOrCreateParametersFromFile(args.rootChParamsFile)
	if err != nil {
		logger.Printf("%v\n", err)
		return err
	}

	miner.Init(validatorPubKey, multisigPubKey, &rootPrivKey.PublicKey, commPrivKey, rootCommPrivKey, rootChParams)
	return nil
}

func (args startArgs) ValidateInput() error {
	if len(args.dbname) == 0 {
		return errors.New("argument missing: dbname")
	}

	if len(args.myNodeAddress) == 0 {
		return errors.New("argument missing: myNodeAddress")
	}

	if len(args.bootstrapNodeAddress) == 0 {
		return errors.New("argument missing: bootstrapNodeAddress")
	}

	if len(args.walletFile) == 0 {
		return errors.New("argument missing: keyFile")
	}

	if len(args.commitmentFile) == 0 {
		return errors.New("argument missing: commitmentFile")
	}

	if len(args.rootWalletFile) == 0 {
		return errors.New("argument missing: rootWalletFile")
	}

	if len(args.rootCommitmentFile) == 0 {
		return errors.New("argument missing: rootCommitmentFile")
	}

	if len(args.rootChParamsFile) == 0 {
		return errors.New("argument missing: rootChParamsFile")
	}

	return nil
}

func (args startArgs) String() string {
	return fmt.Sprintf("Starting bazo miner with arguments \n"+
		"- Database Name:\t\t %v\n"+
		"- My Address:\t\t\t %v\n"+
		"- Bootstrap Address:\t\t %v\n"+
		"- Wallet File:\t\t\t %v\n"+
		"- Multisig File:\t\t %v\n"+
		"- Commitment File:\t\t %v\n"+
		"- Root Wallet File:\t\t %v\n"+
		"- Root Commitment File:\t\t %v\n"+
		"- Root ChParams File:\t\t %v\n",
		args.dbname,
		args.myNodeAddress,
		args.bootstrapNodeAddress,
		args.walletFile,
		args.multisigFile,
		args.commitmentFile,
		args.rootWalletFile,
		args.rootCommitmentFile,
		args.rootChParamsFile,
	)
}
