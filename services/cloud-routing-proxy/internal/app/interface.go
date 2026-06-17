package app

// BalancerEngine задает абстрактные b2b-границы распределенной L7-маршрутизации кластера
type BalancerEngine interface {
	AddNode(node string)
	RouteRoom(roomID string) (string, error)
}
