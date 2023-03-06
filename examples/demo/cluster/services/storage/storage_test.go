package storage

// func TestRedisStore_Put(t *testing.T) {
// 	redisClient := redis.NewClient(&redis.Options{
// 		Addr:     "localhost:6379", // use default Addr
// 		Password: "",               // no password set
// 		DB:       0,                // use default DB
// 	})

// 	redisStorage := NewRedisStorage(redisClient)

// 	type fields struct {
// 		PubSub  PubSub
// 		KvStore KvStore
// 		client  *redis.Client
// 	}
// 	type args struct {
// 		ctx   context.Context
// 		key   KeyType
// 		value interface{}
// 	}
// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			name: "case1",
// 			fields: fields{
// 				PubSub:  redisStorage.PubSub(),
// 				KvStore: redisStorage.KvStore(),
// 				client:  redisClient,
// 			},
// 			args: args{
// 				ctx: context.Background(),
// 				key: "test:stream",
// 				value: &proto.FrameTick{
// 					Tick: 1,
// 					Mts:  float64(time.Now().UnixMicro()),
// 				},
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	s := jsonpb.NewSerializer()
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			r := &RedisStore{
// 				PubSub:  tt.fields.PubSub,
// 				KvStore: tt.fields.KvStore,
// 				client:  tt.fields.client,
// 			}
// 			mVal, err := s.Marshal(tt.args.value)
// 			if err != nil {
// 				t.Errorf("RedisStore.Marshal() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 			if err := r.Put(tt.args.ctx, tt.args.key, 1, mVal); (err != nil) != tt.wantErr {
// 				t.Errorf("RedisStore.Put() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 			result, err := r.Read(tt.args.ctx, tt.args.key, 1)
// 			if err != nil {
// 				t.Errorf("RedisStore.Read() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 			messages := result.([]redis.XMessage)

// 			for _, message := range messages {
// 				ft := &proto.FrameTick{}
// 				err := s.Unmarshal([]byte(message.Values["data"].(string)), ft)
// 				if err != nil {
// 					t.Errorf("RedisStore.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
// 				}
// 			}

// 			fmt.Println("OK")
// 		})
// 	}
// }
