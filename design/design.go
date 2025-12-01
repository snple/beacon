package design

type Node struct {
	ID   string `nson:"id"`
	Name string `nson:"name"`

	Wires []Wire `nson:"wires"`
}

type Wire struct {
	ID       string   `nson:"id"`
	Name     string   `nson:"name"`
	Type     string   `nson:"type"`
	Tags     []string `nson:"tags,omitempty"`
	Clusters []string `nson:"clusters,omitempty"`

	Pins []Pin `nson:"pins"`
}

type Pin struct {
	ID    string   `nson:"id"`
	Name  string   `nson:"name"`
	Addr  string   `nson:"addr"`
	Type  string   `nson:"type"`
	Unit  string   `nson:"unit,omitempty"`
	Scale string   `nson:"scale,omitempty"`
	Tags  []string `nson:"tags,omitempty"`
}
