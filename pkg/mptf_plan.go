package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/go-multierror"
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
	for _, t := range m.Transforms {
		if err = golden.Decode(t); err != nil {
			err = multierror.Append(err, fmt.Errorf("%s(%s) decode error: %+v", t.Address(), t.HclBlock().Range().String(), err))
		}
		if err != nil {
			return err
		}
	}

	for _, t := range m.Transforms {
		if applyErr := t.Apply(); applyErr != nil {
			err = multierror.Append(err, applyErr)
		}
	}
	if err != nil {
		return fmt.Errorf("errors applying transforms: %+v", err)
	}
	if err = m.c.SaveToDisk(); err != nil {
		return fmt.Errorf("errors saving changes: %+v", err)
	}

	return nil
}
