package device

// ToStorageNode 将 Device 模板转换为可存储的 Node 配置
// 这个辅助函数方便将设备模板应用到实际的节点配置中
//
// 注意：返回的结构体中 ID、Name、Status、Updated 等字段需要由调用方设置
func (d *Device) ToStorageNode() map[string]interface{} {
	wires := make([]map[string]interface{}, 0, len(d.Wires))

	for _, w := range d.Wires {
		pins := make([]map[string]interface{}, 0, len(w.Pins))

		for _, p := range w.Pins {
			pin := map[string]interface{}{
				"name": p.Name,
				"type": p.Type,
				"rw":   p.Rw,
			}

			// 只有非空字段才添加
			if p.Desc != "" {
				pin["desc"] = p.Desc
			}
			if len(p.Tags) > 0 {
				pin["tags"] = p.Tags
			}

			pins = append(pins, pin)
		}

		wire := map[string]interface{}{
			"name": w.Name,
			"pins": pins,
		}

		// 只有非空字段才添加
		if w.Desc != "" {
			wire["desc"] = w.Desc
		}
		if w.Type != "" {
			wire["type"] = w.Type
		}
		if len(w.Tags) > 0 {
			wire["tags"] = w.Tags
		}

		wires = append(wires, wire)
	}

	return map[string]interface{}{
		"wires": wires,
	}
}

// ClonePins 克隆 Pin 列表（深拷贝）
func ClonePins(pins []Pin) []Pin {
	result := make([]Pin, len(pins))
	for i, p := range pins {
		result[i] = Pin{
			Name: p.Name,
			Desc: p.Desc,
			Type: p.Type,
			Rw:   p.Rw,
		}
		// Tags 深拷贝
		if len(p.Tags) > 0 {
			result[i].Tags = make([]string, len(p.Tags))
			copy(result[i].Tags, p.Tags)
		}
		// Default 值不拷贝，保持引用
		result[i].Default = p.Default
	}
	return result
}

// CloneWires 克隆 Wire 列表（深拷贝）
func CloneWires(wires []Wire) []Wire {
	result := make([]Wire, len(wires))
	for i, w := range wires {
		result[i] = Wire{
			Name: w.Name,
			Desc: w.Desc,
			Type: w.Type,
			Pins: ClonePins(w.Pins),
		}
		// Tags 深拷贝
		if len(w.Tags) > 0 {
			result[i].Tags = make([]string, len(w.Tags))
			copy(result[i].Tags, w.Tags)
		}
	}
	return result
}
