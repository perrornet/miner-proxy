package handles

import (
	"reflect"
	"testing"
)

func TestZipParams_build(t *testing.T) {
	type fields struct {
		ClientSystemType   string
		ClientSystemStruct string
		ClientRunType      string
		Forward            []Forward
	}
	type args struct {
		filename   string
		secretKey  string
		serverPort string
		serverHost string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			name: "windows-amd64-frontend",
			fields: fields{
				ClientSystemType:   "windows",
				ClientSystemStruct: "amd64",
				ClientRunType:      "frontend",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte(".\\test.file -k 123456789 -r 127.0.0.1:2356 -l :8080 -c -u baidu.com:80\npause"),
		},
		{
			name: "windows-amd64-backend",
			fields: fields{
				ClientSystemType:   "windows",
				ClientSystemStruct: "amd64",
				ClientRunType:      "backend",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte{64, 101, 99, 104, 111, 32, 111, 102, 102, 10, 105, 102, 32, 34, 37, 49, 34, 61, 61, 34, 104, 34, 32, 103, 111, 116, 111, 32, 98, 101, 103, 105, 110, 10, 10, 115, 116, 97, 114, 116, 32, 109, 115, 104, 116, 97, 32, 118, 98, 115, 99, 114, 105, 112, 116, 58, 99, 114, 101, 97, 116, 101, 111, 98, 106, 101, 99, 116, 40, 34, 119, 115, 99, 114, 105, 112, 116, 46, 115, 104, 101, 108, 108, 34, 41, 46, 114, 117, 110, 40, 34, 34, 34, 37, 126, 110, 120, 48, 34, 34, 32, 104, 34, 44, 48, 41, 40, 119, 105, 110, 100, 111, 119, 46, 99, 108, 111, 115, 101, 41, 38, 38, 101, 120, 105, 116, 10, 10, 58, 98, 101, 103, 105, 110, 10, 10, 46, 92, 116, 101, 115, 116, 46, 102, 105, 108, 101, 32, 45, 107, 32, 49, 50, 51, 52, 53, 54, 55, 56, 57, 32, 45, 114, 32, 49, 50, 55, 46, 48, 46, 48, 46, 49, 58, 50, 51, 53, 54, 32, 45, 108, 32, 58, 56, 48, 56, 48, 32, 45, 99, 32, 45, 117, 32, 98, 97, 105, 100, 117, 46, 99, 111, 109, 58, 56, 48},
		},
		{
			name: "windows-amd64-service",
			fields: fields{
				ClientSystemType:   "windows",
				ClientSystemStruct: "amd64",
				ClientRunType:      "service",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte(".\\test.file -k 123456789 -r 127.0.0.1:2356 -l :8080 -c -u baidu.com:80 install\npause"),
		},

		// linux
		{
			name: "linux-amd64-frontend",
			fields: fields{
				ClientSystemType:   "linux",
				ClientSystemStruct: "amd64",
				ClientRunType:      "frontend",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte("sudo su\nchmod +x ./test.file\n./test.file -k 123456789 -r 127.0.0.1:2356 -l :8080 -c -u baidu.com:80"),
		},
		{
			name: "linux-amd64-backend",
			fields: fields{
				ClientSystemType:   "linux",
				ClientSystemStruct: "amd64",
				ClientRunType:      "backend",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte("sudo su\nchmod +x ./test.file\nnohup ./test.file -k 123456789 -r 127.0.0.1:2356 -l :8080 -c -u baidu.com:80 > ./miner-proxy.log 2>& 1&"),
		},
		{
			name: "linux-amd64-service",
			fields: fields{
				ClientSystemType:   "linux",
				ClientSystemStruct: "amd64",
				ClientRunType:      "service",
				Forward: []Forward{
					{
						Port: "8080",
						Pool: "baidu.com:80",
					},
				},
			},
			args: args{
				filename:   "test.file",
				secretKey:  "123456789",
				serverPort: "2356",
				serverHost: "127.0.0.1",
			},
			want: []byte("sudo su\nchmod +x ./test.file\n./test.file -k 123456789 -r 127.0.0.1:2356 -l :8080 -c -u baidu.com:80 install"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := ZipParams{
				ClientSystemType:   tt.fields.ClientSystemType,
				ClientSystemStruct: tt.fields.ClientSystemStruct,
				ClientRunType:      tt.fields.ClientRunType,
				Forward:            tt.fields.Forward,
			}
			if got := z.build(tt.args.filename, tt.args.secretKey, tt.args.serverPort, tt.args.serverHost); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("build() = %s, want %s", got, tt.want)
			}
		})
	}
}
