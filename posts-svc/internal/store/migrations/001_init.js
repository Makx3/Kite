db.posts.createIndex({ user_id: 1, created_at: -1 }, { name: "posts_user_created" });
db.posts.createIndex({ created_at: -1 }, { name: "posts_created" });
