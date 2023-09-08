package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"os"
	"strings"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""

	app *cli.App

	// Flags needed by sigen
	abiFlag = cli.StringFlag{
		Name:  "abi",
		Usage: "Path to the Ethereum contract ABI json to bind, - for STDIN",
	}
	pkgFlag = cli.StringFlag{
		Name:  "pkg",
		Usage: "Package name to generate the binding into",
	}
	typeFlag = cli.StringFlag{
		Name:  "type",
		Usage: "Struct name for the binding (default = package name)",
	}
	outFlag = cli.StringFlag{
		Name:  "out",
		Usage: "Output file for the generated binding (default = stdout)",
	}
)

func sigen(c *cli.Context) error {
	utils.CheckExclusive(c, abiFlag) // Only one source can be selected.
	if c.GlobalString(pkgFlag.Name) == "" {
		utils.Fatalf("No destination package specified (--pkg)")
	}

	var lang bind.Lang = bind.LangGo

	// If the entire solidity code was specified, build and bind based on that
	var (
		abis    []string
		bins    []string
		types   []string
		sigs    []map[string]string
		libs    = make(map[string]string)
		aliases = make(map[string]string)
	)
	if c.GlobalString(abiFlag.Name) != "" {
		// Load up the ABI, optional bytecode and type name from the parameters
		var (
			abi []byte
			err error
		)
		input := c.GlobalString(abiFlag.Name)
		if input == "-" {
			abi, err = ioutil.ReadAll(os.Stdin)
		} else {
			abi, err = ioutil.ReadFile(input)
		}
		if err != nil {
			utils.Fatalf("Failed to read input ABI: %v", err)
		}
		abis = append(abis, string(abi))

		var bin []byte
		bins = append(bins, string(bin))

		kind := c.GlobalString(typeFlag.Name)
		if kind == "" {
			kind = c.GlobalString(pkgFlag.Name)
		}
		types = append(types, kind)
	} else {
		// Generate the list of types to exclude from binding
		exclude := make(map[string]bool)
		var contracts map[string]*compiler.Contract

		// Gather all non-excluded contract for binding
		for name, contract := range contracts {
			if exclude[strings.ToLower(name)] {
				continue
			}
			abi, err := json.Marshal(contract.Info.AbiDefinition) // Flatten the compiler parse
			if err != nil {
				utils.Fatalf("Failed to parse ABIs from compiler output: %v", err)
			}
			abis = append(abis, string(abi))
			bins = append(bins, contract.Code)
			sigs = append(sigs, contract.Hashes)
			nameParts := strings.Split(name, ":")
			types = append(types, nameParts[len(nameParts)-1])

			libPattern := crypto.Keccak256Hash([]byte(name)).String()[2:36]
			libs[libPattern] = nameParts[len(nameParts)-1]
		}
	}

	// Generate the contract binding
	globalString := c.GlobalString(pkgFlag.Name)
	code, err := bind.Pack(types, abis, bins, sigs, globalString, lang, libs, aliases)
	if err != nil {
		utils.Fatalf("Failed to generate ABI binding: %v", err)
	}
	// Either flush it out to a file or display on the standard output
	if !c.GlobalIsSet(outFlag.Name) {
		fmt.Printf("%s\n", code)
		return nil
	}
	if err := ioutil.WriteFile(c.GlobalString(outFlag.Name), []byte(code), 0600); err != nil {
		utils.Fatalf("Failed to write ABI binding: %v", err)
	}
	return nil
}

func init() {
	app = flags.NewApp(gitCommit, gitDate, "ethereum checkpoint helper tool")
	app.Flags = []cli.Flag{
		abiFlag,
		pkgFlag,
		outFlag,
	}
	app.Action = utils.MigrateFlags(sigen)
	cli.CommandHelpTemplate = flags.OriginCommandHelpTemplate
}

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
