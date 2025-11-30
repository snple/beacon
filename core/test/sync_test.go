package test

import (
	"database/sql"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

var _ = ginkgo.Describe("Test core sync API", ginkgo.Label("library"), func() {
	var db *bun.DB
	var cs *core.CoreService

	ginkgo.BeforeEach(func() {
		sqlite, err := sql.Open("sqlite3", ":memory:")
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		db = bun.NewDB(sqlite, sqlitedialect.New())

		// db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))

		err = core.CreateSchema(db)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		cs, err = core.Core(db)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		cs.Start()
	})

	ginkgo.AfterEach(func() {
		c := make(chan bool)
		go func() {
			cs.Stop()
			close(c)
		}()
		gomega.Eventually(c, "1s").Should(gomega.BeClosed())
	})

	ginkgo.Context("test notify", func() {
		id := util.RandomID()

		ginkgo.It("create", func(ctx ginkgo.SpecContext) {

			notify := cs.GetSync().Notify(id, core.NOTIFY)
			defer notify.Close()

			n := 0

			select {
			case <-notify.Wait():
				n += 1
			default:
			}

			notify2 := cs.GetSync().Notify(id, core.NOTIFY)
			defer notify2.Close()

			n2 := 0

			select {
			case <-notify2.Wait():
				n2 += 1
			default:
			}

			notify3 := cs.GetSync().Notify("123", core.NOTIFY)
			defer notify2.Close()

			n3 := 0

			select {
			case <-notify3.Wait():
				n3 += 1
			default:
			}

			gomega.Expect(n3 == 1).To(gomega.Equal(true))

			{
				request := &pb.Node{
					Id:     id,
					Name:   "test_node1",
					Secret: "123456",
					Status: consts.ON,
				}

				reply, err := cs.GetNode().Create(ctx, request)

				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				_ = reply
			}

			{
				request := &pb.Name{Name: "test_node1"}

				reply, err := cs.GetNode().Name(ctx, request)

				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				_ = reply
			}

			select {
			case <-notify.Wait():
				n += 1
			default:
			}

			gomega.Expect(n == 2).To(gomega.Equal(true))

			select {
			case <-notify.Wait():
				n += 1
			default:
			}

			gomega.Expect(n == 2).To(gomega.Equal(true))

			select {
			case <-notify2.Wait():
				n2 += 1
			default:
			}

			gomega.Expect(n2 == 2).To(gomega.Equal(true))

			select {
			case <-notify3.Wait():
				n3 += 1
			default:
			}

			gomega.Expect(n3 == 1).To(gomega.Equal(true))
		})
	})
})
