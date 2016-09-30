package deployer_test

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("Deployer", func() {
// 	var sut *deployer.Deployer
// 	var redisConn *redigomock.Conn
// 	var etcdClient *FakeEtcdClient
//
// 	BeforeEach(func() {
// 		httpmock.Activate()
// 		redisConn = redigomock.NewConn()
// 		etcdClient = &FakeEtcdClient{}
// 		sut = deployer.New(etcdClient, redisConn, "redis-queue:name", "https://deploy-state.test", "super")
// 	})
//
// 	AfterEach(func() {
// 		httpmock.DeactivateAndReset()
// 	})
//
// 	It("Should exist", func() {
// 		Expect(sut).NotTo(BeNil())
// 	})
//
// 	Describe("Run", func() {
// 		var err error
//
// 		Describe("When there are no pending deploys", func() {
// 			BeforeEach(func() {
// 				now := time.Now().Unix()
//
// 				redisConn.Command("ZRANGEBYSCORE", "redis-queue:name:governator:deploys", 0, now).Expect([]interface{}{})
// 				err = sut.Run()
// 			})
//
// 			It("Should return right away without an error", func() {
// 				Expect(err).To(BeNil())
// 			})
// 		})
//
// 		Describe("When ZRANGEBYSCORE returns an error", func() {
// 			BeforeEach(func() {
// 				now := time.Now().Unix()
// 				redisConn.Command("ZRANGEBYSCORE", "redis-queue:name:governator:deploys", 0, now).ExpectError(fmt.Errorf("things went worse than expected"))
// 				err = sut.Run()
// 			})
//
// 			It("Should return right away with the error", func() {
// 				Expect(err).To(MatchError("things went worse than expected"))
// 			})
// 		})
//
// 		Describe("When there are is a pending deploy that should not go out yet", func() {
// 			BeforeEach(func() {
// 				now := time.Now().Unix()
// 				redisConn.Command("ZRANGEBYSCORE", "redis-queue:name:governator:deploys", 0, now).Expect([]interface{}{})
// 				err = sut.Run()
// 			})
//
// 			It("Should return right away without an error", func() {
// 				Expect(err).To(BeNil())
// 			})
// 		})
//
// 		Describe("When there are is a pending deploy that should go out", func() {
// 			BeforeEach(func() {
// 				now := time.Now().Unix()
// 				pendingDeploys := make([]interface{}, 1)
// 				pendingDeploys[0] = []byte("pending-deploy-1")
// 				redisConn.Command("ZRANGEBYSCORE", "redis-queue:name:governator:deploys", 0, now).Expect(pendingDeploys)
// 			})
//
// 			Describe("When attempting to ZREM the record yields no change", func() {
// 				var zrem *redigomock.Cmd
//
// 				BeforeEach(func() {
// 					zrem = redisConn.Command("ZREM", "redis-queue:name:governator:deploys", "pending-deploy-1").Expect(int64(0))
// 					sut.Run()
// 				})
//
// 				It("Should return try to delete the record", func() {
// 					Expect(redisConn.Stats(zrem)).To(Equal(1), "ZREM was not called enough times")
// 				})
// 			})
//
// 			Describe("When attempting to ZREM yields an error", func() {
// 				BeforeEach(func() {
// 					redisConn.Command("ZREM", "redis-queue:name:governator:deploys", "pending-deploy-1").ExpectError(fmt.Errorf("something wong"))
// 					err = sut.Run()
// 				})
//
// 				It("Should return try to delete the record", func() {
// 					Expect(err).To(MatchError("something wong"))
// 				})
// 			})
//
// 			Describe("When attempting to ZREM the record succeeds", func() {
// 				BeforeEach(func() {
// 					redisConn.Command("ZREM", "redis-queue:name:governator:deploys", "pending-deploy-1").Expect(int64(1))
// 				})
//
// 				Describe("When the deploy has been cancelled", func() {
// 					BeforeEach(func() {
// 						redisConn.Command("HEXISTS", "redis-queue:name:pending-deploy-1", "cancellation").Expect(int64(1))
// 						err = sut.Run()
// 					})
//
// 					It("Should return with a nil error", func() {
// 						Expect(err).To(BeNil())
// 					})
// 				})
//
// 				Describe("When the deploy not been cancelled", func() {
// 					BeforeEach(func() {
// 						redisConn.Command("HEXISTS", "redis-queue:name:pending-deploy-1", "cancellation").Expect(int64(0))
// 					})
//
// 					Describe("When the metadata doesn't exist", func() {
// 						BeforeEach(func() {
// 							redisConn.Command("HGET", "redis-queue:name:pending-deploy-1", "request:metadata").Expect(nil)
// 							err = sut.Run()
// 						})
//
// 						It("Should return an error", func() {
// 							Expect(err).To(MatchError("Deploy metadata not found for 'pending-deploy-1'"))
// 						})
// 					})
//
// 					Describe("When the metadata exists", func() {
// 						BeforeEach(func() {
// 							cmd := redisConn.Command("HGET", "redis-queue:name:pending-deploy-1", "request:metadata")
// 							cmd.Expect([]byte("{\"etcdDir\":\"/octoblu/my-application\", \"dockerUrl\":\"octoblu/my-application:v1\"}"))
// 							rsp := httpmock.NewStringResponder(200, "Ok")
// 							httpmock.RegisterResponder("PUT", "https://deploy-state.test/deployments/octoblu/my-application/v1/cluster/super/passed", rsp)
// 							err = sut.Run()
// 						})
//
// 						It("Should update the application's docker url", func() {
// 							firstCall := etcdClient.SetCalls[0]
// 							Expect(firstCall[0]).To(Equal("/octoblu/my-application/docker_url"))
// 							Expect(firstCall[1]).To(Equal("octoblu/my-application:v1"))
// 						})
//
// 						It("Should update the application's sentry release", func() {
// 							secondCall := etcdClient.SetCalls[1]
// 							Expect(secondCall[0]).To(Equal("/octoblu/my-application/env/SENTRY_RELEASE"))
// 							Expect(secondCall[1]).To(Equal("v1"))
// 						})
//
// 						It("Should touch restart", func() {
// 							thirdCall := etcdClient.SetCalls[2]
// 							Expect(thirdCall[0]).To(Equal("/octoblu/my-application/restart"))
// 							Expect(thirdCall[1]).NotTo(BeNil())
// 						})
// 					})
// 				})
//
// 				Describe("When the deploy not been cancelled, but etcd Set returns an error", func() {
// 					BeforeEach(func() {
// 						redisConn.Command("HEXISTS", "redis-queue:name:pending-deploy-1", "cancellation").Expect(int64(0))
// 						cmd := redisConn.Command("HGET", "redis-queue:name:pending-deploy-1", "request:metadata")
// 						cmd.Expect([]byte("{\"etcdDir\":\"/octoblu/my-application\", \"dockerUrl\":\"octoblu/my-application:version\"}"))
//
// 						etcdClient.SetError = fmt.Errorf("The server is gone, url is wrong, etc(d)...")
// 						err = sut.Run()
// 					})
//
// 					It("Should error", func() {
// 						Expect(err).To(MatchError("The server is gone, url is wrong, etc(d)..."))
// 					})
// 				})
// 			})
// 		})
// 	})
// })
//
// type FakeEtcdClient struct {
// 	SetCalls [][]string
// 	SetError error
// }
//
// func (etcdClient *FakeEtcdClient) Get(string) (string, error) {
// 	return "", nil
// }
//
// func (etcdClient *FakeEtcdClient) Set(key, value string) error {
// 	etcdClient.SetCalls = append(etcdClient.SetCalls, []string{key, value})
//
// 	return etcdClient.SetError
// }
