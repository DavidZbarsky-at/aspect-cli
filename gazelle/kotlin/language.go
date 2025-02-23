package gazelle

import (
	"aspect.build/cli/gazelle/common/git"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const LanguageName = "kotlin"

const (
	KtJvmLibrary              = "kt_jvm_library"
	RulesKotlinRepositoryName = "io_bazel_rules_kotlin"
)

type Java_PackageName struct {
	Name string
}
type Java_MavenResolver interface {
	Resolve(pkg Java_PackageName) (label.Label, error)
}

// The Gazelle extension for TypeScript rules.
// TypeScript satisfies the language.Language interface including the
// Configurer and Resolver types.
type kotlinLang struct {
	config.Configurer
	resolve.Resolver

	// Ignore configurations for the workspace.
	gitignore *git.GitIgnore
}

// NewLanguage initializes a new TypeScript that satisfies the language.Language
// interface. This is the entrypoint for the extension initialization.
func NewLanguage() language.Language {
	l := &kotlinLang{
		gitignore: git.NewGitIgnore(),
	}

	l.Configurer = NewConfigurer(l)
	l.Resolver = NewResolver(l)

	return l
}

var kotlinKinds = map[string]rule.KindInfo{
	KtJvmLibrary: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs: map[string]bool{
			"srcs": true,
		},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
}

var kotlinLoads = []rule.LoadInfo{
	{
		Name: "@" + RulesKotlinRepositoryName + "//kotlin:jvm.bzl",
		Symbols: []string{
			KtJvmLibrary,
		},
	},
}

func (*kotlinLang) Kinds() map[string]rule.KindInfo {
	return kotlinKinds
}

func (*kotlinLang) Loads() []rule.LoadInfo {
	return kotlinLoads
}

func (*kotlinLang) Fix(c *config.Config, f *rule.File) {}
