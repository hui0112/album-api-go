from locust import FastHttpUser, task, between

class MyUser(FastHttpUser):
    wait_time = between(1, 2)

    @task(3)
    def read_albums(self):
        # 对应你的 router.GET("/albums", getAlbums)
        self.client.get("/albums")

    @task(1)
    def post_album(self):
        # 对应你的 router.POST("/albums", postAlbums)
        # 发送一个假的 JSON 数据
        self.client.post("/albums", json={
            "id": "100",
            "title": "Test Album",
            "artist": "Test Artist",
            "price": 49.99
        })