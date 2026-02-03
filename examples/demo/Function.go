/*original:
package demo;

func FunctionHi(data string) string {
  return "hi " + data;
}

func FunctionYo(data string) string {
  return data + " yoyoyo";
} 
generated:
*/
package demo;

import "context"

/*
func FunctionHi(data string) string {
  return "hi " + data;
} 
*/
func FunctionHi(ctx context.Context, data string) string {
  ctx, cancel := context.WithCancel(ctx)
	defer cancel()
  return "hi " + data;
}

/*
func FunctionYo(data string) string {
  return data + " yoyoyo";
} 
*/
func FunctionYo(ctx context.Context, data string) string {
  ctx, cancel := context.WithCancel(ctx)
	defer cancel()
  return data + " yoyoyo";
}
