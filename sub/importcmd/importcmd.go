// Copyright (c) 2022 Wireleap

package importcmd

import (
	"flag"
	"text/tabwriter"

	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Import accesskeys JSON and set up associated contract",
		Sections: []cli.Section{{
			Title: "Arguments",
			Entries: []cli.Entry{
				{Key: "FILE", Value: "Path to accesskeys file, or - to read standard input"},
				{Key: "URL", Value: "URL to download accesskeys (https required)"},
			},
		}},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 8, ' ', 0)
	r.Run = func(fm fsdir.T) {
		/*
			c := clientcfg.Defaults()
			err := fm.Get(&c, filenames.Config)

			if err != nil {
				log.Fatal(err)
			}

			if fs.NArg() != 1 {
				r.Usage()
				os.Exit(1)
			}

			akfile := fs.Arg(0)
			data := []byte{}

			switch {
			case akfile == "-":
				data, err = ioutil.ReadAll(os.Stdin)
			case strings.HasPrefix(akfile, "http://"):
				log.Fatal("HTTP import URLs are vulnerable to MitM attacks. Use HTTPS instead.")
			case strings.HasPrefix(akfile, "https://"):
				data, err = download(akfile)
			default:
				data, err = ioutil.ReadFile(akfile)
			}

			if err != nil {
				log.Fatalf("could not read accesskey file: %s", err)
			}

			ak := &accesskey.T{}
			err = json.Unmarshal(data, &ak)

			if err != nil {
				log.Fatal("could not unmarshal accesskey file: ", err)
			}

			switch {
			case ak == nil,
				ak.Version == nil,
				ak.Contract == nil,
				ak.Pofs == nil,
				ak.Contract.Endpoint == nil,
				ak.Contract.PublicKey == nil:
				log.Fatal("malformed accesskey file")
			}

			if ak.Version.Minor != accesskey.VERSION.Minor {
				log.Fatalf(
					"incompatible accesskey version: %s, expected 0.%d.x",
					ak.Version,
					accesskey.VERSION.Minor,
				)
			}

			sc0 := clientlib.ContractURL(fm)
			if sc0 != nil && *sc0 != *ak.Contract.Endpoint {
				log.Fatalf(
					"you are trying to import accesskeys for a contract %s different from the currently defined %s; please set up a separate wireleap directory for your %s needs and import %s accesskeys there",
					ak.Contract.Endpoint,
					sc0,
					ak.Contract.Endpoint,
					akfile,
				)
			}

			cl := client.New(nil, clientcontract.T)
			ci, d, err := clientlib.GetContractInfo(cl, ak.Contract.Endpoint)

			if err != nil {
				log.Fatalf(
					"could not get contract info for %s: %s",
					ak.Contract.Endpoint, err,
				)
			}

			if !bytes.Equal(ak.Contract.PublicKey, ci.Pubkey) {
				log.Fatalf(
					"contract public key mismatch; expecting %s from accesskey file, got %s from live contract",
					ak.Contract.PublicKey,
					base64.RawURLEncoding.EncodeToString(ci.Pubkey),
				)
			}

			if err = clientlib.SaveContractInfo(fm, ci, d); err != nil {
				log.Fatalf(
					"could not save contract info for %s: %s",
					ak.Contract.Endpoint,
					err,
				)
			}
			sc := ci.Endpoint
			pofs := []*pof.T{}
			if err = fm.Get(&pofs, filenames.Pofs); errors.Is(err, io.EOF) || errors.Is(err, os.ErrNotExist) {
				// this is fine
				err = nil
			}
			if err != nil {
				log.Fatalf(
					"could not get previous pofs for %s: %s",
					sc.String(),
					err,
				)
			}

			for _, p := range ak.Pofs {
				if p.Expiration <= time.Now().Unix() {
					log.Printf("skipping expired accesskey %s", p.Digest())
					continue
				}
				dup := false
				for _, p0 := range pofs {
					if p0.Digest() == p.Digest() {
						log.Printf("skipping duplicate accesskey %s", p.Digest())
						dup = true
						break
					}
				}
				if !dup {
					pofs = append(pofs, p)
				}
			}

			if err = fm.Set(pofs, filenames.Pofs); err != nil {
				log.Fatalf(
					"could not save new pofs for %s: %s",
					sc.String(), err,
				)
			}
			di, err := consume.DirectoryInfo(cl, sc)
			if err != nil {
				log.Fatalf("could not get contract directory info: %s", err)
			}
			// maybe there's an upgrade available?
			var upgradev *semver.Version
			if di.UpgradeChannels.Client != nil {
				if v, ok := di.UpgradeChannels.Client[version.Channel]; ok && v.GT(version.VERSION) {
					upgradev = &v
				}
			}
			var pid int
			if err = fm.Get(&pid, filenames.Pid); err == nil {
				if upgradev != nil {
					log.Printf(
						"Upgrade available to %s, current version is %s. Please run `wireleap upgrade`.",
						upgradev, version.VERSION,
					)
				}
				process.Reload(pid)
			}
		*/
	}
	r.SetMinimalUsage("FILE|URL")
	return r
}
