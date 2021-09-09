package coin

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/go-resty/resty/v2"
	"github.com/gobenpark/trader/item"
)

type Upbit struct {
	*resty.Client
}

func NewStore() *Upbit {
	client := resty.New()

	client.SetHostURL("https://api.upbit.com/v1")
	return &Upbit{client}
}

func (u Upbit) GetMarketItems() []item.Item {
	res, err := u.Client.R().Get("/market/all?isDetails=false")
	if err != nil {
		return nil
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(res.Body(), &data); err != nil {
		fmt.Println(err)
		return nil
	}

	var items []item.Item
	for _, i := range data {
		if !regexp.MustCompile("^KRW+").MatchString(i["market"].(string)) {
			continue
		}
		items = append(items, item.Item{
			Code: i["market"].(string),
			Name: i["korean_name"].(string),
			Tag: func() string {
				if i["market_warning"] != nil {
					return i["market_warning"].(string)
				}
				return ""
			}(),
		})
	}
	return items
}
