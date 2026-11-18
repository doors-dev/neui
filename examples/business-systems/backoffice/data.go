package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/doors-dev/doors"
)

type status string

const (
	paid      status = "paid"
	unpaid    status = "unpaid"
	refunded  status = "refunded"
	cancelled status = "cancelled"
)

var statuses = []status{paid, unpaid, refunded, cancelled}

type customer struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Address string `json:"address" description:"default shipping address"`
	Notes   string `json:"notes"`
}

type product struct {
	ID    int     `json:"id"`
	SKU   string  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price" description:"USD per unit"`
	Stock int     `json:"stock" description:"units in stock"`
}

type line struct {
	ProductID int     `json:"product_id"`
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price" description:"USD per unit at order time"`
}

type order struct {
	ID         int     `json:"id"`
	CustomerID int     `json:"customer_id"`
	Customer   string  `json:"customer"`
	Date       string  `json:"date" description:"YYYY-MM-DD"`
	Status     status  `json:"status"`
	Address    string  `json:"address" description:"shipping address"`
	Lines      []line  `json:"lines"`
	Total      float64 `json:"total" description:"USD, sum of the lines"`
}

type orderQuery struct {
	Status   status `json:"status,omitempty" description:"paid, unpaid, refunded or cancelled; empty for all"`
	From     string `json:"from,omitempty" description:"first order date, YYYY-MM-DD; empty for no limit"`
	To       string `json:"to,omitempty" description:"last order date, YYYY-MM-DD; empty for no limit"`
	Search   string `json:"search,omitempty" description:"matches order id, customer name or email, or shipping address"`
	Customer int    `json:"customer,omitempty" description:"customer id; 0 for all"`
	Sort     string `json:"sort,omitempty" description:"date, total, customer or id; prefix - for descending; default -date"`
}

type customerRow struct {
	customer
	Orders int     `json:"orders" description:"orders that are not cancelled"`
	Spent  float64 `json:"spent" description:"USD paid, refunds excluded"`
}

type fieldErrors map[string]string

func (e fieldErrors) err() error {
	if len(e) == 0 {
		return nil
	}
	var parts []string
	for _, key := range slices.Sorted(maps.Keys(e)) {
		if key == "" {
			parts = append(parts, e[key])
			continue
		}
		parts = append(parts, key+": "+e[key])
	}
	return errors.New(strings.Join(parts, " "))
}

type shopData struct {
	mu        sync.Mutex
	orders    []order
	customers []customer
	products  []product
	nextID    struct{ order, customer, product int }
	version   doors.Source[int]
}

var shop = newShop()

func round(v float64) float64 {
	return math.Round(v*100) / 100
}

func newShop() *shopData {
	customers := []customer{
		{1, "Ada Lovelace", "ada@analytical.co.uk", "+44 20 7946 0101", "Analytical Engines Ltd", "12 St James's Square, London SW1Y 4JH", "Prefers morning deliveries."},
		{2, "Alan Turing", "alan@bletchley.org", "+44 1625 555 019", "Bletchley Research", "8 Adlington Road, Wilmslow SK9 2BJ", ""},
		{3, "Grace Hopper", "grace@cobol.dev", "+1 212 555 0143", "Compiler Corp", "350 5th Ave, New York, NY 10118", "Office orders, invoice monthly."},
		{4, "Linus Torvalds", "linus@kernel.example", "+1 503 555 0188", "", "1120 NW Couch St, Portland, OR 97209", ""},
		{5, "Margaret Hamilton", "margaret@apollo.space", "+1 617 555 0112", "Apollo Guidance", "77 Massachusetts Ave, Cambridge, MA 02139", ""},
		{6, "Ken Thompson", "ken@unix.example", "+1 408 555 0170", "Bell Coffee Club", "170 W Tasman Dr, San Jose, CA 95134", "Leave at reception."},
		{7, "Barbara Liskov", "barbara@clu.edu", "+1 617 555 0175", "", "32 Vassar St, Cambridge, MA 02139", ""},
		{8, "Donald Knuth", "don@taocp.example", "+1 650 555 0132", "Stanford Typesetting", "450 Jane Stanford Way, Stanford, CA 94305", "Pays by cheque, slow but steady."},
		{9, "Radia Perlman", "radia@spanningtree.net", "+1 206 555 0156", "Bridge Networks", "400 Broad St, Seattle, WA 98109", ""},
		{10, "Edsger Dijkstra", "edsger@shortest.path", "+1 512 555 0199", "", "2317 Speedway, Austin, TX 78712", ""},
		{11, "Frances Allen", "fran@optimize.dev", "+1 914 555 0128", "Yorktown Roasters", "1101 Kitchawan Rd, Yorktown Heights, NY 10598", "Wholesale account."},
		{12, "John McCarthy", "john@lisp.example", "+1 650 555 0107", "", "353 Jane Stanford Way, Stanford, CA 94305", ""},
	}
	products := []product{
		{1, "SKU-101", "Espresso cup", 9.50, 42},
		{2, "SKU-102", "Pour-over kettle", 64.00, 7},
		{3, "SKU-103", "Burr grinder", 189.00, 0},
		{4, "SKU-104", "Travel mug", 24.90, 18},
		{5, "SKU-105", "Paper filters (100)", 6.50, 120},
		{6, "SKU-106", "Digital scale", 39.00, 4},
		{7, "SKU-107", "Cold brew jar", 29.00, 25},
		{8, "SKU-108", "Milk frothing pitcher", 18.00, 33},
	}
	r := rand.New(rand.NewPCG(7, 11))
	today := time.Now()
	statusAt := func(daysAgo int) status {
		n := r.IntN(20)
		switch {
		case daysAgo < 6 && n < 8:
			return unpaid
		case n < 3:
			return unpaid
		case n < 5:
			return refunded
		case n < 6:
			return cancelled
		}
		return paid
	}
	var orders []order
	for i := range 40 {
		daysAgo := (39 - i) * 2
		if daysAgo > 0 {
			daysAgo -= r.IntN(2)
		}
		c := customers[r.IntN(len(customers))]
		o := order{
			ID:         1001 + i,
			CustomerID: c.ID,
			Date:       today.AddDate(0, 0, -daysAgo).Format(time.DateOnly),
			Status:     statusAt(daysAgo),
			Address:    c.Address,
		}
		used := map[int]bool{}
		for range 1 + r.IntN(3) {
			p := products[r.IntN(len(products))]
			if used[p.ID] {
				continue
			}
			used[p.ID] = true
			o.Lines = append(o.Lines, line{ProductID: p.ID, SKU: p.SKU, Name: p.Name, Qty: 1 + r.IntN(3), Price: p.Price})
		}
		orders = append(orders, o)
	}
	orders[36].CustomerID = 2
	orders[36].Address = "8 Adlingtn Rd, Wilmslo SK9"
	orders[36].Status = unpaid
	s := &shopData{
		orders:    orders,
		customers: customers,
		products:  products,
		version:   doors.NewSource(0),
	}
	s.nextID.order = 1041
	s.nextID.customer = 13
	s.nextID.product = 9
	return s
}

func (s *shopData) bump(ctx context.Context) {
	s.version.Mutate(ctx, func(v int) int { return v + 1 })
}

func (s *shopData) view(o order) order {
	o.Lines = slices.Clone(o.Lines)
	o.Total = 0
	for _, l := range o.Lines {
		o.Total += float64(l.Qty) * l.Price
	}
	o.Total = round(o.Total)
	if i := slices.IndexFunc(s.customers, func(c customer) bool { return c.ID == o.CustomerID }); i >= 0 {
		o.Customer = s.customers[i].Name
	}
	return o
}

func (s *shopData) Orders(q orderQuery) []order {
	s.mu.Lock()
	defer s.mu.Unlock()
	search := strings.ToLower(strings.TrimSpace(q.Search))
	var out []order
	for _, raw := range s.orders {
		o := s.view(raw)
		if q.Status != "" && o.Status != q.Status {
			continue
		}
		if q.From != "" && o.Date < q.From {
			continue
		}
		if q.To != "" && o.Date > q.To {
			continue
		}
		if q.Customer != 0 && o.CustomerID != q.Customer {
			continue
		}
		if search != "" {
			email := ""
			if i := slices.IndexFunc(s.customers, func(c customer) bool { return c.ID == o.CustomerID }); i >= 0 {
				email = s.customers[i].Email
			}
			hay := strings.ToLower(strconv.Itoa(o.ID) + " " + o.Customer + " " + email + " " + o.Address)
			if !strings.Contains(hay, strings.TrimPrefix(search, "#")) {
				continue
			}
		}
		out = append(out, o)
	}
	key, desc := sortKey(q.Sort, "-date")
	slices.SortStableFunc(out, func(a, b order) int {
		var c int
		switch key {
		case "total":
			c = cmp.Compare(a.Total, b.Total)
		case "customer":
			c = cmp.Compare(a.Customer, b.Customer)
		case "id":
			c = cmp.Compare(a.ID, b.ID)
		default:
			c = cmp.Or(cmp.Compare(a.Date, b.Date), cmp.Compare(a.ID, b.ID))
		}
		if desc {
			return -c
		}
		return c
	})
	return out
}

func sortKey(sort string, def string) (string, bool) {
	if sort == "" {
		sort = def
	}
	return strings.TrimPrefix(sort, "-"), strings.HasPrefix(sort, "-")
}

func checkSort(sort string, keys ...string) error {
	if sort == "" || slices.Contains(keys, strings.TrimPrefix(sort, "-")) {
		return nil
	}
	return fmt.Errorf("unknown sort %q; use %s, with - in front for descending", sort, strings.Join(keys, ", "))
}

func (s *shopData) Order(id int) (order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.orders, func(o order) bool { return o.ID == id })
	if i < 0 {
		return order{}, false
	}
	return s.view(s.orders[i]), true
}

func (s *shopData) Customers(search string, sort string) []customerRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	search = strings.ToLower(strings.TrimSpace(search))
	var out []customerRow
	for _, c := range s.customers {
		hay := strings.ToLower(c.Name + " " + c.Email + " " + c.Company + " " + c.Phone)
		if search != "" && !strings.Contains(hay, search) {
			continue
		}
		row := customerRow{customer: c}
		for _, raw := range s.orders {
			if raw.CustomerID != c.ID || raw.Status == cancelled {
				continue
			}
			o := s.view(raw)
			row.Orders++
			if o.Status == paid {
				row.Spent += o.Total
			}
		}
		row.Spent = round(row.Spent)
		out = append(out, row)
	}
	key, desc := sortKey(sort, "name")
	slices.SortStableFunc(out, func(a, b customerRow) int {
		var c int
		switch key {
		case "orders":
			c = cmp.Compare(a.Orders, b.Orders)
		case "spent":
			c = cmp.Compare(a.Spent, b.Spent)
		default:
			c = cmp.Compare(a.Name, b.Name)
		}
		if desc {
			return -c
		}
		return c
	})
	return out
}

func (s *shopData) Customer(id int) (customerRow, bool) {
	s.mu.Lock()
	i := slices.IndexFunc(s.customers, func(c customer) bool { return c.ID == id })
	s.mu.Unlock()
	if i < 0 {
		return customerRow{}, false
	}
	for _, row := range s.Customers("", "") {
		if row.ID == id {
			return row, true
		}
	}
	return customerRow{}, false
}

func (s *shopData) AllCustomers() []customer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.customers)
	slices.SortFunc(out, func(a, b customer) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

func (s *shopData) Products(search string, sort string) []product {
	s.mu.Lock()
	defer s.mu.Unlock()
	search = strings.ToLower(strings.TrimSpace(search))
	var out []product
	for _, p := range s.products {
		if search != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.SKU), search) {
			continue
		}
		out = append(out, p)
	}
	key, desc := sortKey(sort, "name")
	slices.SortStableFunc(out, func(a, b product) int {
		var c int
		switch key {
		case "price":
			c = cmp.Compare(a.Price, b.Price)
		case "stock":
			c = cmp.Compare(a.Stock, b.Stock)
		case "sku":
			c = cmp.Compare(a.SKU, b.SKU)
		default:
			c = cmp.Compare(a.Name, b.Name)
		}
		if desc {
			return -c
		}
		return c
	})
	return out
}

func (s *shopData) Product(id int) (product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.products, func(p product) bool { return p.ID == id })
	if i < 0 {
		return product{}, false
	}
	return s.products[i], true
}

const lowStock = 5

type customerForm struct {
	Name    string `form:"name"`
	Email   string `form:"email"`
	Phone   string `form:"phone"`
	Company string `form:"company"`
	Address string `form:"address"`
	Notes   string `form:"notes"`
}

func (c customer) form() customerForm {
	return customerForm{Name: c.Name, Email: c.Email, Phone: c.Phone, Company: c.Company, Address: c.Address, Notes: c.Notes}
}

func (s *shopData) SaveCustomer(ctx context.Context, id int, f customerForm) (int, fieldErrors) {
	c := customer{
		ID:      id,
		Name:    strings.TrimSpace(f.Name),
		Email:   strings.TrimSpace(f.Email),
		Phone:   strings.TrimSpace(f.Phone),
		Company: strings.TrimSpace(f.Company),
		Address: strings.TrimSpace(f.Address),
		Notes:   strings.TrimSpace(f.Notes),
	}
	errs := fieldErrors{}
	if c.Name == "" {
		errs["name"] = "Enter the customer's name."
	}
	if c.Email == "" {
		errs["email"] = "Enter an email address."
	} else if a, err := mail.ParseAddress(c.Email); err != nil || a.Address != c.Email || !strings.Contains(c.Email[strings.Index(c.Email, "@"):], ".") {
		errs["email"] = "This doesn't look like an email address, e.g. name@example.com."
	}
	if c.Phone != "" && strings.Trim(c.Phone, "+0123456789 ()-") != "" {
		errs["phone"] = "Use digits, spaces, +, ( ) and - only."
	}
	if c.Address == "" {
		errs["address"] = "Enter a shipping address."
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Email != "" && slices.ContainsFunc(s.customers, func(o customer) bool {
		return o.ID != id && strings.EqualFold(o.Email, c.Email)
	}) {
		errs["email"] = "Another customer already uses this email."
	}
	if len(errs) > 0 {
		return 0, errs
	}
	if id == 0 {
		c.ID = s.nextID.customer
		s.nextID.customer++
		s.customers = append(s.customers, c)
	} else {
		i := slices.IndexFunc(s.customers, func(o customer) bool { return o.ID == id })
		if i < 0 {
			return 0, fieldErrors{"": "This customer no longer exists."}
		}
		s.customers[i] = c
	}
	defer s.bump(ctx)
	return c.ID, nil
}

func (s *shopData) DeleteCustomer(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, o := range s.orders {
		if o.CustomerID == id {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("This customer has %d order%s, so they can't be deleted. Orders are kept for the books.", n, plural(n))
	}
	s.customers = slices.DeleteFunc(s.customers, func(c customer) bool { return c.ID == id })
	defer s.bump(ctx)
	return nil
}

type productForm struct {
	SKU   string `form:"sku"`
	Name  string `form:"name"`
	Price string `form:"price"`
	Stock string `form:"stock"`
}

func (p product) form() productForm {
	return productForm{SKU: p.SKU, Name: p.Name, Price: strconv.FormatFloat(p.Price, 'f', 2, 64), Stock: strconv.Itoa(p.Stock)}
}

func (s *shopData) SaveProduct(ctx context.Context, id int, f productForm) (int, fieldErrors) {
	p := product{ID: id, SKU: strings.ToUpper(strings.TrimSpace(f.SKU)), Name: strings.TrimSpace(f.Name)}
	errs := fieldErrors{}
	if p.SKU == "" {
		errs["sku"] = "Enter a SKU."
	} else if strings.ContainsAny(p.SKU, " \t") {
		errs["sku"] = "A SKU has no spaces, e.g. SKU-109."
	}
	if p.Name == "" {
		errs["name"] = "Enter the product name."
	}
	price, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(f.Price), "$"), 64)
	switch {
	case strings.TrimSpace(f.Price) == "":
		errs["price"] = "Enter a price."
	case err != nil:
		errs["price"] = "Enter a number, e.g. 24.90."
	case price <= 0:
		errs["price"] = "The price must be more than zero."
	}
	p.Price = round(price)
	stock, err := strconv.Atoi(strings.TrimSpace(f.Stock))
	switch {
	case strings.TrimSpace(f.Stock) == "":
		errs["stock"] = "Enter the units in stock, 0 if none."
	case err != nil:
		errs["stock"] = "Enter a whole number."
	case stock < 0:
		errs["stock"] = "Stock can't be negative."
	}
	p.Stock = stock
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.SKU != "" && slices.ContainsFunc(s.products, func(o product) bool { return o.ID != id && o.SKU == p.SKU }) {
		errs["sku"] = "Another product already uses this SKU."
	}
	if len(errs) > 0 {
		return 0, errs
	}
	if id == 0 {
		p.ID = s.nextID.product
		s.nextID.product++
		s.products = append(s.products, p)
	} else {
		i := slices.IndexFunc(s.products, func(o product) bool { return o.ID == id })
		if i < 0 {
			return 0, fieldErrors{"": "This product no longer exists."}
		}
		s.products[i] = p
	}
	defer s.bump(ctx)
	return p.ID, nil
}

func (s *shopData) AdjustStock(ctx context.Context, id int, change int) (product, error) {
	p, ok := s.Product(id)
	if !ok {
		return product{}, fmt.Errorf("no product %d", id)
	}
	f := p.form()
	f.Stock = strconv.Itoa(p.Stock + change)
	if _, errs := s.SaveProduct(ctx, id, f); len(errs) > 0 {
		return product{}, errs.err()
	}
	p, _ = s.Product(id)
	return p, nil
}

func (s *shopData) DeleteProduct(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, o := range s.orders {
		if slices.ContainsFunc(o.Lines, func(l line) bool { return l.ProductID == id }) {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("This product is on %d order%s, so it can't be deleted. Set its stock to 0 to stop selling it.", n, plural(n))
	}
	s.products = slices.DeleteFunc(s.products, func(p product) bool { return p.ID == id })
	defer s.bump(ctx)
	return nil
}

type lineForm struct {
	Product string `form:"product"`
	Qty     string `form:"qty"`
}

type orderForm struct {
	Customer string     `form:"customer"`
	Address  string     `form:"address"`
	Status   string     `form:"status"`
	Lines    []lineForm `form:"lines"`
	Intent   string     `form:"intent"`
}

func (o order) form() orderForm {
	f := orderForm{Customer: strconv.Itoa(o.CustomerID), Address: o.Address, Status: string(o.Status)}
	for _, l := range o.Lines {
		f.Lines = append(f.Lines, lineForm{Product: strconv.Itoa(l.ProductID), Qty: strconv.Itoa(l.Qty)})
	}
	return f
}

func (s *shopData) ChangeOrder(ctx context.Context, id int, change func(f *orderForm) error) error {
	o, ok := s.Order(id)
	if !ok {
		return fmt.Errorf("no order %d", id)
	}
	f := o.form()
	if err := change(&f); err != nil {
		return err
	}
	_, errs := s.SaveOrder(ctx, id, f)
	return errs.err()
}

func (s *shopData) SaveOrder(ctx context.Context, id int, f orderForm) (int, fieldErrors) {
	errs := fieldErrors{}
	s.mu.Lock()
	defer s.mu.Unlock()
	var old order
	if id != 0 {
		i := slices.IndexFunc(s.orders, func(o order) bool { return o.ID == id })
		if i < 0 {
			return 0, fieldErrors{"": "This order no longer exists."}
		}
		old = s.orders[i]
		if old.Status == refunded || old.Status == cancelled {
			return 0, fieldErrors{"": "Refunded and cancelled orders can't be changed."}
		}
	}
	o := order{ID: id, Date: old.Date, Address: strings.TrimSpace(f.Address), Status: status(f.Status)}
	cid, _ := strconv.Atoi(f.Customer)
	ci := slices.IndexFunc(s.customers, func(c customer) bool { return c.ID == cid })
	if ci < 0 {
		errs["customer"] = "Pick a customer."
	} else {
		o.CustomerID = cid
		if o.Address == "" {
			o.Address = s.customers[ci].Address
		}
	}
	if o.Status != paid && o.Status != unpaid {
		errs["status"] = "Pick paid or unpaid."
	}
	stock := map[int]int{}
	for _, p := range s.products {
		stock[p.ID] = p.Stock
	}
	for _, l := range old.Lines {
		stock[l.ProductID] += l.Qty
	}
	seen := map[int]bool{}
	for i, lf := range f.Lines {
		key := fmt.Sprintf("lines.%d", i)
		pid, _ := strconv.Atoi(lf.Product)
		pi := slices.IndexFunc(s.products, func(p product) bool { return p.ID == pid })
		if pi < 0 {
			errs[key+".product"] = "Pick a product."
			continue
		}
		p := s.products[pi]
		if seen[pid] {
			errs[key+".product"] = "This product is already on the order; raise the quantity there."
			continue
		}
		seen[pid] = true
		qty, err := strconv.Atoi(strings.TrimSpace(lf.Qty))
		switch {
		case err != nil || qty < 1:
			errs[key+".qty"] = "At least 1."
			continue
		case qty > stock[pid]:
			errs[key+".qty"] = fmt.Sprintf("Only %d in stock.", stock[pid])
			continue
		}
		price := p.Price
		if j := slices.IndexFunc(old.Lines, func(l line) bool { return l.ProductID == pid }); j >= 0 {
			price = old.Lines[j].Price
		}
		o.Lines = append(o.Lines, line{ProductID: pid, SKU: p.SKU, Name: p.Name, Qty: qty, Price: price})
	}
	if len(f.Lines) == 0 {
		errs["lines"] = "Add at least one product."
	}
	if len(errs) > 0 {
		return 0, errs
	}
	for _, l := range old.Lines {
		s.restock(l.ProductID, l.Qty)
	}
	for _, l := range o.Lines {
		s.restock(l.ProductID, -l.Qty)
	}
	if id == 0 {
		o.ID = s.nextID.order
		s.nextID.order++
		o.Date = time.Now().Format(time.DateOnly)
		s.orders = append(s.orders, o)
	} else {
		i := slices.IndexFunc(s.orders, func(x order) bool { return x.ID == id })
		s.orders[i] = o
	}
	defer s.bump(ctx)
	return o.ID, nil
}

func (s *shopData) restock(pid int, qty int) {
	for i := range s.products {
		if s.products[i].ID == pid {
			s.products[i].Stock += qty
		}
	}
}

func (s *shopData) Refund(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.orders, func(o order) bool { return o.ID == id })
	if i < 0 || s.orders[i].Status != paid {
		return errors.New("only paid orders can be refunded")
	}
	s.orders[i].Status = refunded
	defer s.bump(ctx)
	return nil
}

func (s *shopData) Cancel(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.orders, func(o order) bool { return o.ID == id })
	if i < 0 || s.orders[i].Status != unpaid {
		return errors.New("only unpaid orders can be cancelled; refund a paid one")
	}
	s.orders[i].Status = cancelled
	for _, l := range s.orders[i].Lines {
		s.restock(l.ProductID, l.Qty)
	}
	defer s.bump(ctx)
	return nil
}

type stats struct {
	Revenue      float64   `json:"revenue_this_month" description:"USD of paid orders dated this month"`
	RevenuePrev  float64   `json:"revenue_last_month"`
	Orders       int       `json:"orders_this_month"`
	Unpaid       int       `json:"unpaid_orders"`
	UnpaidAmount float64   `json:"unpaid_amount" description:"USD"`
	LowStock     []product `json:"low_stock" description:"products with 5 or fewer units"`
}

func (s *shopData) Stats() stats {
	now := time.Now()
	month := now.Format("2006-01")
	prev := now.AddDate(0, -1, -now.Day()+1).Format("2006-01")
	var st stats
	for _, o := range s.Orders(orderQuery{}) {
		switch {
		case o.Status == unpaid:
			st.Unpaid++
			st.UnpaidAmount += o.Total
		}
		if strings.HasPrefix(o.Date, month) {
			if o.Status != cancelled {
				st.Orders++
			}
			if o.Status == paid {
				st.Revenue += o.Total
			}
		}
		if strings.HasPrefix(o.Date, prev) && o.Status == paid {
			st.RevenuePrev += o.Total
		}
	}
	st.Revenue, st.RevenuePrev, st.UnpaidAmount = round(st.Revenue), round(st.RevenuePrev), round(st.UnpaidAmount)
	for _, p := range s.Products("", "stock") {
		if p.Stock <= lowStock {
			st.LowStock = append(st.LowStock, p)
		}
	}
	return st
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func money(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	whole := int64(v)
	cents := int64(math.Round((v - float64(whole)) * 100))
	if cents == 100 {
		whole++
		cents = 0
	}
	digits := strconv.FormatInt(whole, 10)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	out := fmt.Sprintf("$%s.%02d", b.String(), cents)
	if neg {
		return "-" + out
	}
	return out
}
