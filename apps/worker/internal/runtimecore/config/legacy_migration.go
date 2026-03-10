package config

import (
	"gopkg.in/yaml.v3"
)

// removeLegacyOpenAICompatAPIKeys strips the deprecated "api-keys" field from each
// openai-compatibility entry in the raw YAML tree.
func removeLegacyOpenAICompatAPIKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	idx := findMapKeyIndex(root, "openai-compatibility")
	if idx < 0 || idx+1 >= len(root.Content) {
		return
	}
	seq := root.Content[idx+1]
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return
	}
	for i := range seq.Content {
		if seq.Content[i] != nil && seq.Content[i].Kind == yaml.MappingNode {
			removeMapKey(seq.Content[i], "api-keys")
		}
	}
}

// removeLegacyAmpKeys strips deprecated top-level amp-* keys from the raw YAML tree.
func removeLegacyAmpKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "amp-upstream-url")
	removeMapKey(root, "amp-upstream-api-key")
	removeMapKey(root, "amp-restrict-management-to-localhost")
	removeMapKey(root, "amp-model-mappings")
}

// removeLegacyGenerativeLanguageKeys strips the deprecated generative-language-api-key
// list from the raw YAML tree.
func removeLegacyGenerativeLanguageKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "generative-language-api-key")
}

// removeLegacyAuthBlock strips the deprecated top-level auth block from the raw YAML tree.
func removeLegacyAuthBlock(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "auth")
}
