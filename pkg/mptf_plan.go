package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"strings"
)

var _ golden.Plan = &MetaProgrammingTFPlan{}

func RunMetaProgrammingTFPlan(c *MetaProgrammingTFConfig) (*MetaProgrammingTFPlan, error) {
	if err := c.RunPlan(); err != nil {
		return nil, err
	}
	plan := &MetaProgrammingTFPlan{
		c: c,
	}
	plan.Transforms = append(plan.Transforms, golden.Blocks[Transform](c)...)
	return plan, nil
}

type MetaProgrammingTFPlan struct {
	c          *MetaProgrammingTFConfig
	Transforms []Transform
}

func (m *MetaProgrammingTFPlan) String() string {
	sb := strings.Builder{}
	for _, t := range m.Transforms {
		sb.WriteString(fmt.Sprintf("%s would be apply:\n %s\n", t.Address(), golden.BlockToString(t)))
		sb.WriteString("\n---\n")
	}
	return sb.String()
}

func (m *MetaProgrammingTFPlan) Apply() error {
	var err error

	if err = golden.Traverse[Transform](m.c.BaseConfig, func(b Transform) error {
		if err := golden.Decode(b); err != nil {
			return fmt.Errorf("%s(%s) decode error: %+v", b.Address(), b.HclBlock().Range().String(), err)
		}
		return nil
	}); err != nil {
		return err
	}

	if err = golden.Traverse[Transform](m.c.BaseConfig, func(b Transform) error {
		return b.Apply()
	}); err != nil {
		return fmt.Errorf("errors applying transforms: %+v", err)
	}
	if err = m.c.SaveToDisk(); err != nil {
		return fmt.Errorf("errors saving changes: %+v", err)
	}

	return nil
}
