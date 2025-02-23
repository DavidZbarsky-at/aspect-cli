package gazelle

import (
	"fmt"
	"math"
	"os"
	"path"
	"sync"

	gazelle "aspect.build/cli/gazelle/common"
	. "aspect.build/cli/gazelle/common/log"
	"aspect.build/cli/gazelle/kotlin/kotlinconfig"
	"aspect.build/cli/gazelle/kotlin/parser"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/sets/treeset"
)

const (
	// TODO: move to common
	MaxWorkerCount = 12
)

func (kt *kotlinLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	BazelLog.Tracef("GenerateRules '%s'", args.Rel)

	// TODO: record args.GenFiles labels?

	cfg := args.Config.Exts[LanguageName].(kotlinconfig.Configs)[args.Rel]

	// TODO: exit if configured to disable generation

	// Collect all source files.
	sourceFiles := kt.collectSourceFiles(cfg, args)

	// TODO: multiple targets (lib, test, ...)
	target := NewKotlinTarget()

	// Parse all source files and group information into target(s)
	for p := range kt.parseFiles(args, sourceFiles) {
		target.Files.Add(p.File)
		target.Packages.Add(p.Package)

		for _, impt := range p.Imports {
			target.Imports.Add(ImportStatement{
				ImportSpec: resolve.ImportSpec{
					Lang: LanguageName,
					Imp:  impt,
				},
				SourcePath: p.File,
			})
		}
	}

	var result language.GenerateResult

	targetName := gazelle.ToDefaultTargetName(args, "root")

	kt.addLibraryRule(targetName, target, args, false, &result)

	return result
}

func (kt *kotlinLang) addLibraryRule(targetName string, target *KotlinTarget, args language.GenerateArgs, isTestRule bool, result *language.GenerateResult) {
	// TODO: check for rule collisions

	// Generate nothing if there are no source files. Remove any existing rules.
	if target.Files.Empty() {
		if args.File == nil {
			return
		}

		for _, r := range args.File.Rules {
			if r.Name() == targetName && r.Kind() == KtJvmLibrary {
				emptyRule := rule.NewRule(KtJvmLibrary, targetName)
				result.Empty = append(result.Empty, emptyRule)
				return
			}
		}

		return
	}

	ktLibrary := rule.NewRule(KtJvmLibrary, targetName)
	ktLibrary.SetAttr("srcs", target.Files.Values())
	ktLibrary.SetPrivateAttr(packagesKey, target)

	if isTestRule {
		ktLibrary.SetAttr("testonly", true)
	}

	result.Gen = append(result.Gen, ktLibrary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktLibrary.Kind(), args.Rel, ktLibrary.Name())
}

// TODO: put in common?
func (kt *kotlinLang) parseFiles(args language.GenerateArgs, sources *treeset.Set) chan *parser.ParseResult {
	// The channel of all files to parse.
	sourcePathChannel := make(chan string)

	// The channel of parse results.
	resultsChannel := make(chan *parser.ParseResult)

	// The number of workers. Don't create more workers than necessary.
	workerCount := int(math.Min(MaxWorkerCount, float64(1+sources.Size()/2)))

	// Start the worker goroutines.
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for sourcePath := range sourcePathChannel {
				r, errs := parseFile(path.Join(args.Config.RepoRoot, args.Rel), sourcePath)

				// Output errors to stdout
				if len(errs) > 0 {
					fmt.Println(sourcePath, "parse error(s):")
					for _, err := range errs {
						fmt.Println(err)
					}
				}

				resultsChannel <- r
			}
		}()
	}

	// Send files to the workers.
	go func() {
		sourceFileChannelIt := sources.Iterator()
		for sourceFileChannelIt.Next() {
			sourcePathChannel <- sourceFileChannelIt.Value().(string)
		}

		close(sourcePathChannel)
	}()

	// Wait for all workers to finish.
	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	return resultsChannel
}

// Parse the passed file for import statements.
func parseFile(rootDir, filePath string) (*parser.ParseResult, []error) {
	BazelLog.Debugf("ParseImports: %s", filePath)

	content, err := os.ReadFile(path.Join(rootDir, filePath))
	if err != nil {
		return nil, []error{err}
	}

	p := parser.NewParser()
	return p.Parse(filePath, string(content))
}

func (kt *kotlinLang) collectSourceFiles(cfg *kotlinconfig.KotlinConfig, args language.GenerateArgs) *treeset.Set {
	sourceFiles := treeset.NewWithStringComparator()

	// TODO: "module" targets similar to java?

	gazelle.GazelleWalkDir(args, false, func(f string) error {
		// Globally managed file ignores.
		if kt.gitignore.Matches(path.Join(args.Rel, f)) {
			BazelLog.Tracef("File git ignored: %s / %s", args.Rel, f)

			return nil
		}

		// Otherwise the file is either source or potentially importable.
		if isSourceFileType(f) {
			BazelLog.Tracef("SourceFile: %s", f)

			sourceFiles.Add(f)
		}

		return nil
	})

	return sourceFiles
}

func isSourceFileType(f string) bool {
	ext := path.Ext(f)
	return ext == ".kt" || ext == ".kts"
}
