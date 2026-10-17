package help

// api (api_search.cmd_api): the useful public surface of a class, its own
// slots, new methods and overrides, without the framework base classes'
// methods.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// frameworkClasses are _FRAMEWORK_CLASSES: base classes whose methods are
// noise in an API view.
var frameworkClasses = map[string]bool{
	"BObject": true, "BValue": true, "BComplex": true, "BComponent": true, "BStruct": true,
	"BEnum": true, "BFrozenEnum": true, "BDynamicEnum": true,
	"Object": true, "java.lang.Object": true,
}

// APIParams are the arguments of `api`.
type APIParams struct {
	Name      string
	Inherited bool // also the methods inherited from non-framework parents
}

// API is api_search.cmd_api.
func (e *Env) API(p APIParams) error {
	classes, err := e.loadIndex("class-index.json", false)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	methods, err := e.loadIndex("method-index.json", false)
	if err != nil {
		return err
	}
	slots, err := e.loadIndex("slots-index.json", false)
	if err != nil {
		return err
	}
	hits := classMatches(classes, p.Name)
	if len(hits) == 0 {
		return e.reportMiss("api", p.Name, "classes")
	}
	name, pkg := hits[0].Name, hits[0].Package
	chain := ancestry(inh, name)
	framework := map[string]bool{}
	for _, a := range chain[1:] {
		if frameworkClasses[a] {
			framework[a] = true
		}
	}
	cm := classMethods(methods, chain)
	mine := cm[name]
	parents := parentMethods(cm, chain[1:])
	var own, overridden, inherited []string
	for _, m := range sortedKeys(mine) {
		if _, ok := parents[m]; ok {
			overridden = append(overridden, m)
		} else {
			own = append(own, m)
		}
	}
	if p.Inherited {
		for _, m := range sortedKeys(parents) {
			if _, ok := mine[m]; !ok && !framework[parents[m].class] {
				inherited = append(inherited, m)
			}
		}
	}
	filtered := 0
	for a := range framework {
		for m := range cm[a] {
			if _, ok := mine[m]; !ok {
				filtered++
			}
		}
	}

	fmt.Fprintf(e.Out, "API: %s (%s)\n%s\n", name, pkg, strings.Repeat("=", 60))
	entry := inh.Object(name)
	if ext := truthyStr(entry, "extends"); ext != "" {
		fmt.Fprintf(e.Out, "  extends %s\n", ext)
	}
	if impl := strList(entry, "implements"); len(impl) > 0 {
		fmt.Fprintf(e.Out, "  implements %s\n", strings.Join(impl, ", "))
	}
	data := slots.Object(name)
	props, actions, topics := objects(data, "properties"), objects(data, "actions"), objects(data, "topics")
	totalSlots := len(props) + len(actions) + len(topics)
	if totalSlots > 0 {
		fmt.Fprintf(e.Out, "\nSLOTS (%d props, %d actions, %d topics):\n", len(props), len(actions), len(topics))
		for _, s := range props {
			fmt.Fprintf(e.Out, "  [P] %s: %s%s%s\n", getStr(s, "name", "?"), getStr(s, "type", "?"),
				defaultSuffix(s, 50), flagsSuffix(s))
		}
		for _, a := range actions {
			fmt.Fprintf(e.Out, "  [A] %s%s\n", getStr(a, "name", "?"), paramSuffix(a))
		}
		for _, t := range topics {
			fmt.Fprintf(e.Out, "  [T] %s%s\n", getStr(t, "name", "?"), eventSuffix(t))
		}
	}
	if len(own) > 0 {
		fmt.Fprintf(e.Out, "\nOWN METHODS (%d):\n\n", len(own))
		for _, m := range own {
			fmt.Fprintf(e.Out, "  %s\n", mine[m])
		}
	}
	if len(overridden) > 0 {
		fmt.Fprintf(e.Out, "\nOVERRIDES (%d):\n\n", len(overridden))
		for _, m := range overridden {
			fmt.Fprintf(e.Out, "  %s (from %s)\n", mine[m], parents[m].class)
		}
	}
	if len(inherited) > 0 {
		fmt.Fprintf(e.Out, "\nINHERITED (non-framework) (%d):\n\n", len(inherited))
		e.printByOrigin(inherited, parents)
	}
	fmt.Fprintf(e.Out, "\nSUMMARY: %d own + %d overrides = %d useful\n", len(own), len(overridden), len(own)+len(overridden))
	if filtered > 0 {
		fmt.Fprintf(e.Out, "  (%d framework methods from %s filtered out)\n", filtered,
			strings.Join(sortedKeys(framework), ", "))
	}
	if len(own) == 0 && len(overridden) == 0 && totalSlots == 0 {
		fmt.Fprintln(e.Out, "  (no own methods or slots found)")
	}
	return nil
}

// paramSuffix is "(<parameterType>)" when the parameter type is truthy,
// else "()": the action form of api and contract (no space, unlike slots).
func paramSuffix(action *index.Object) string {
	if pt := truthyStr(action, "parameterType"); pt != "" {
		return "(" + pt + ")"
	}
	return "()"
}
