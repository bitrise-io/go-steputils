package keytemplate

import (
	"text/template"
	"text/template/parse"
)

// UsesChecksum reports whether the key template calls checksum, i.e. whether the key changes with the
// content of the files it names. An invalid template reports false.
func UsesChecksum(keyTemplate string) bool {
	funcMap := template.FuncMap{
		"getenv":   func(string) string { return "" },
		"checksum": func(...string) string { return "" },
	}

	tmpl, err := template.New("").Funcs(funcMap).Parse(keyTemplate)
	if err != nil || tmpl.Tree == nil {
		return false
	}

	return callsChecksum(tmpl.Tree.Root)
}

func callsChecksum(node parse.Node) bool {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return false
		}
		for _, child := range n.Nodes {
			if callsChecksum(child) {
				return true
			}
		}
	case *parse.ActionNode:
		return callsChecksum(n.Pipe)
	case *parse.PipeNode:
		if n == nil {
			return false
		}
		for _, cmd := range n.Cmds {
			if callsChecksum(cmd) {
				return true
			}
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			if callsChecksum(arg) {
				return true
			}
		}
	case *parse.ChainNode:
		return callsChecksum(n.Node)
	case *parse.IdentifierNode:
		return n.Ident == "checksum"
	case *parse.IfNode:
		return branchCallsChecksum(&n.BranchNode)
	case *parse.RangeNode:
		return branchCallsChecksum(&n.BranchNode)
	case *parse.WithNode:
		return branchCallsChecksum(&n.BranchNode)
	}

	return false
}

func branchCallsChecksum(n *parse.BranchNode) bool {
	return callsChecksum(n.Pipe) || callsChecksum(n.List) || callsChecksum(n.ElseList)
}
