package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Calcium-Ion/moejs"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2singbox/httputils"
)

// maxScriptBytes 是后处理脚本的大小上限，直链与内联参数共用。
const maxScriptBytes = 1 << 20

// scriptTimeout 是单个脚本的执行时限，超时即中断，避免死循环拖垮请求。
const scriptTimeout = 2 * time.Second

// scriptMemoryLimit 是单次执行允许分配的内存，防止脚本把进程堆撑爆。
const scriptMemoryLimit = 64 << 20

// errScriptResult 表示脚本返回了 JSON 对象以外的东西。
var errScriptResult = errors.New("后处理脚本的返回值必须是 JSON 对象")

// postprocess 在配置生成后运行用户脚本，返回修改后的配置。
//
// 脚本以 (function (config) { ... }) 的形式执行，config 是完整的 sing-box 配置对象，
// 可以就地修改，也可以返回一个新对象替换它；返回 undefined 表示沿用就地修改后的 config。
// 脚本可用 console.log 等输出调试信息，会写进服务端日志。
func (c *Convert) postprocess(ctx context.Context, hc *http.Client, arg model.ConvertArg, config map[string]any) (map[string]any, error) {
	script, err := c.loadScript(ctx, hc, arg)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}
	// 包成函数，脚本里的 return 才不会在顶层报语法错误。
	fn, err := moejs.CompileScript("postprocess.js", "(function (config) {\n"+string(script)+"\n})")
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}

	rt := moejs.NewRuntime(moejs.Options{
		DisableDynamicCode: true, // 脚本无需 eval，关掉可缩小攻击面
		MemoryLimit:        scriptMemoryLimit,
	})
	if err := rt.SetGlobal("console", c.console(rt)); err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}
	fnVal, err := rt.RunScript(fn)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}
	// 前面的 SetGlobal / RunScript 会消耗内存预算，调用前重置，让脚本拿到完整额度。
	rt.ResetAllocation()
	argVal, err := rt.FromGo(config)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(runCtx, func() {
		rt.Interrupt(context.Cause(runCtx))
		close(interrupted)
	})
	defer func() {
		if !stop() {
			<-interrupted // Interrupt 必须返回后才能离开 Call
		}
	}()

	res, err := rt.CallFunction(fnVal, moejs.Undefined(), argVal)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}

	target := argVal // 脚本未显式返回时，取就地修改后的 config
	if !res.IsUndefined() {
		target = res
	}
	out, err := rt.ToGo(target)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("postprocess: %w", errScriptResult)
	}
	return m, nil
}

// loadScript 取用脚本：arg.ScriptUrl 优先（远程直链），否则用内联的 arg.Script。
func (c *Convert) loadScript(ctx context.Context, hc *http.Client, arg model.ConvertArg) ([]byte, error) {
	if arg.ScriptUrl != "" {
		// HttpGet 用 LimitReader 截断而非报错，多读一个字节以便发现超限。
		b, err := httputils.HttpGet(ctx, hc, arg.ScriptUrl, maxScriptBytes+1)
		if err != nil {
			return nil, fmt.Errorf("loadScript: %w", err)
		}
		if len(b) > maxScriptBytes {
			return nil, fmt.Errorf("loadScript: 脚本超过 %d 字节", maxScriptBytes)
		}
		return b, nil
	}
	if len(arg.Script) == 0 {
		return nil, errors.New("loadScript: 脚本为空")
	}
	if len(arg.Script) > maxScriptBytes {
		return nil, fmt.Errorf("loadScript: 脚本超过 %d 字节", maxScriptBytes)
	}
	return arg.Script, nil
}

// console 构造脚本可见的 console 对象，输出写进服务端调试日志。
func (c *Convert) console(rt *moejs.Runtime) map[string]any {
	realm := rt.Realm()
	at := func(level string) moejs.NativeFunc {
		return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			parts := make([]string, 0, len(args))
			for _, a := range args {
				if s, err := realm.ToString(a); err == nil {
					parts = append(parts, s.GoString())
				} else {
					parts = append(parts, a.String())
				}
			}
			c.l.Debug("script console." + level + ": " + strings.Join(parts, " "))
			return moejs.Undefined(), nil
		}
	}
	return map[string]any{
		"log":   at("log"),
		"info":  at("info"),
		"warn":  at("warn"),
		"error": at("error"),
		"debug": at("debug"),
	}
}
