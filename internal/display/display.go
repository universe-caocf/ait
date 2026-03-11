package display

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	"github.com/yinxulai/ait/internal/types"
)

// Colors 定义终端颜色 - 导出供外部使用
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

type Input struct {
	TaskId             string // 任务 ID，随机生成的唯一标识符
	Protocol           string
	BaseUrl            string
	ApiKey             string
	Models             []string // 多个模型列表
	Concurrency        int
	Count              int
	Stream             bool
	Thinking           bool   // 是否开启思考模式
	PromptText         string // 用于显示的prompt文本
	PromptShouldTruncate bool   // 是否需要截断显示
	IsFile             bool   // 是否为文件类型输入
	Report             bool   // 是否生成报告文件
	Timeout            int    // 请求超时时间(秒)
}

// Displayer 测试显示器
type Displayer struct {
	progressBar *progressbar.ProgressBar
	mu          sync.Mutex
}

// New 创建新的测试显示器
func New() *Displayer {
	return &Displayer{}
}

func (td *Displayer) ShowWelcome(version string) {
	fmt.Printf("\n")
	// AIT ASCII 字符画和说明信息并排显示，使用统一的青色+粗体
	fmt.Printf("%s%s    █████╗  ██╗ ████████╗%s    %s🚀 %sAI 模型性能测试工具 %s(%s)%s\n", ColorBold, ColorCyan, ColorReset, ColorBold, ColorCyan, ColorGreen, version, ColorReset)
	fmt.Printf("%s%s   ██╔══██╗ ██║ ╚══██╔══╝%s       %s一个强大的 CLI 工具，用于测试 AI 模型的性能指标%s\n", ColorBold, ColorCyan, ColorReset, ColorWhite, ColorReset)
	fmt.Printf("%s%s   ███████║ ██║    ██║%s          %s🌐 项目地址: https://github.com/yinxulai/ait%s\n", ColorBold, ColorCyan, ColorReset, ColorBlue, ColorReset)
	fmt.Printf("%s%s   ██╔══██║ ██║    ██║%s       \n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("%s%s   ██║  ██║ ██║    ██║%s          %s✨ 功能特性:%s\n", ColorBold, ColorCyan, ColorReset, ColorBold, ColorReset)
	fmt.Printf("%s%s   ╚═╝  ╚═╝ ╚═╝    ╚═╝%s          🎯 多模型批量测试  ⚡ 并发压力测试  📊 实时进度显示\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("                                🌐 网络性能分析    📈 详细统计报告  🎨 美观界面输出\n")
	fmt.Printf("\n")
}

func (td *Displayer) ShowInput(data *Input) {
	// 创建配置信息表格
	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithEastAsian(false),
	)
	table.Header("配置项", "值", "说明")

	// 任务信息
	table.Append("🆔 任务 ID", data.TaskId, "本次测试的唯一标识符")

	// 基础配置
	table.Append("🔗 协议", data.Protocol, "API 协议类型")
	table.Append("🌐 服务地址", data.BaseUrl, "API 基础 URL")
	table.Append("🔑 API 密钥", maskApiKey(data.ApiKey), "API 访问密钥（已隐藏）")

	// 模型配置
	modelsStr := ""
	if len(data.Models) > 0 {
		for i, model := range data.Models {
			if i > 0 {
				modelsStr += ", "
			}
			modelsStr += model
		}
	}
	table.Append("🤖 测试模型", modelsStr, "待测试的模型列表")

	// 测试参数
	table.Append("📊 请求总数", strconv.Itoa(data.Count), "每个模型的请求数量")
	table.Append("⚡ 并发数", strconv.Itoa(data.Concurrency), "同时发送的请求数")
	table.Append("🕐 超时时间", strconv.Itoa(data.Timeout)+"秒", "每个请求的超时时间")
	table.Append("🌊 流式模式", strconv.FormatBool(data.Stream), "是否启用流式响应")
	table.Append("🧠 思考模式", strconv.FormatBool(data.Thinking), "是否启用思考模式（仅OpenAI协议支持）")

	// 对于不需要截断的内容（文件或已包含长度信息的生成内容），直接显示
	var promptDisplay string
	if !data.PromptShouldTruncate {
		promptDisplay = data.PromptText
	} else {
		promptDisplay = truncatePrompt(data.PromptText)
	}

	table.Append("📝 测试提示词", promptDisplay, "用于测试的提示内容")

	table.Append("📄 生成报告", strconv.FormatBool(data.Report), "是否生成测试报告文件")

	table.Render()
}

// InitProgress 初始化进度条
func (td *Displayer) InitProgress(total int, description string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.progressBar = progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionThrottle(100), // 限制更新频率
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true),
	)
}

func (td *Displayer) UpdateProgress(percent float64, TPS float64, TTFT time.Duration, QPS float64, Failed int) {
	td.mu.Lock()
	defer td.mu.Unlock()

	if td.progressBar != nil {
		// 计算当前进度值（基于进度条的最大值）
		current := int(percent * float64(td.progressBar.GetMax()) / 100.0)
		td.progressBar.Set(current)

		// 更新指标
		description := fmt.Sprintf("测试进度 | outTPS: %8.2f | TTFT: %v | QPS: %6.2f | Failed: %6d \n", TPS, TTFT, QPS, Failed)
		td.progressBar.Describe(description)
	}
}

// FinishProgress 完成进度条
func (td *Displayer) FinishProgress() {
	td.mu.Lock()
	defer td.mu.Unlock()

	if td.progressBar != nil {
		td.progressBar.Finish()
		fmt.Println() // 添加一个空行
		td.progressBar = nil
	}
}

func (td *Displayer) ShowErrorsReport(errors []*string) {
	if len(errors) == 0 {
		return
	}

	// 统计错误信息和出现次数
	errorCounts := make(map[string]int)
	totalErrors := 0

	for _, errorPtr := range errors {
		if errorPtr != nil {
			errorMsg := *errorPtr
			errorCounts[errorMsg]++
			totalErrors++
		}
	}

	if totalErrors == 0 {
		return
	}

	fmt.Printf("%s%s❌ 错误信息报告%s\n", ColorBold, ColorRed, ColorReset)
	fmt.Printf("   %s检测到 %d 个错误（%d 种不同类型）%s\n\n", ColorYellow, totalErrors, len(errorCounts), ColorReset)

	// 创建错误信息表格
	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithEastAsian(false),
		// tablewriter.WithAutoWrapText(true),
		// tablewriter.WithWrapTextWidth(80),
	)

	table.Header("序号", "错误详情", "出现次数")

	// 添加错误信息到表格
	index := 1
	for errorMsg, count := range errorCounts {
		// 如果错误信息太长，进行适当的截断和格式化
		displayMsg := errorMsg
		if len(displayMsg) > 300 {
			displayMsg = displayMsg[:299] + "..."
		}
		table.Append(fmt.Sprintf("%d", index), displayMsg, fmt.Sprintf("%d", count))
		index++
	}

	table.Render()
	fmt.Println()
}

// 将数据更新到终端上（刷新显示）
// 详细模式，展示所有 ReportData 的数据
func (td *Displayer) ShowSignalReport(data *types.ReportData) {
	// 单个综合表格
	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithEastAsian(false),
	)

	table.Header("指标", "最小值", "平均值", "标准差", "最大值", "单位", "采样方式说明")

	// 基础信息（这些只有单一值，只填最小值列）
	table.Append("🔗 协议", data.Protocol, "", "", "", "-", "配置信息")
	table.Append("🤖 模型", data.Model, "", "", "", "-", "配置信息")
	table.Append("🌐 URL", data.BaseUrl, "", "", "", "-", "配置信息")
	table.Append("🌊 流式", strconv.FormatBool(data.IsStream), "", "", "", "-", "配置信息")
	table.Append("🧠 思考模式", strconv.FormatBool(data.IsThinking), "", "", "", "-", "配置信息")
	table.Append("⚡ 并发数", strconv.Itoa(data.Concurrency), "", "", "", "个", "配置信息")
	table.Append("📊 总请求数", strconv.Itoa(data.TotalRequests), "", "", "", "个", "完成的请求总数")
	table.Append("✅ 成功率", fmt.Sprintf("%.2f", data.SuccessRate), "", "", "", "%", "成功请求占比")

	// 时间性能指标
	table.Append("🕐 总耗时", formatDuration(data.MinTotalTime), formatDuration(data.AvgTotalTime), fmt.Sprintf("±%s", formatDuration(data.StdDevTotalTime)), formatDuration(data.MaxTotalTime), "时间", "单个请求从发起到完全结束的时间")

	if data.TargetIP != "" {
		table.Append("🎯 目标 IP", data.TargetIP, "", "", "", "-", "DNS 解析后的实际连接 IP")
	}
	// 内容性能指标
	if data.IsStream {
		table.Append("⚡ TTFT", formatDuration(data.MinTTFT), formatDuration(data.AvgTTFT), fmt.Sprintf("±%s", formatDuration(data.StdDevTTFT)), formatDuration(data.MaxTTFT), "时间", "首个 token 响应时间 (含请求发送+网络+服务器处理)")
		table.Append("⚡ TPOT", formatDuration(data.MinTPOT), formatDuration(data.AvgTPOT), fmt.Sprintf("±%s", formatDuration(data.StdDevTPOT)), formatDuration(data.MaxTPOT), "时间", "每个输出 token 的平均耗时 (除首token外)")
	}

	// 网络性能指标
	table.Append("🔍 DNS 时间", formatDuration(data.MinDNSTime), formatDuration(data.AvgDNSTime), "", formatDuration(data.MaxDNSTime), "时间", "域名解析耗时 (httptrace)")
	table.Append("🔒 TLS 时间", formatDuration(data.MinTLSHandshakeTime), formatDuration(data.AvgTLSHandshakeTime), "", formatDuration(data.MaxTLSHandshakeTime), "时间", "TLS 握手耗时 (httptrace)")
	table.Append("🔌 TCP 连接时间", formatDuration(data.MinConnectTime), formatDuration(data.AvgConnectTime), "", formatDuration(data.MaxConnectTime), "时间", "TCP 连接建立耗时 (httptrace)")

	table.Append("🚀 输出 TPS", fmt.Sprintf("%.2f", data.MinTPS), fmt.Sprintf("%.2f", data.AvgTPS), fmt.Sprintf("±%.2f", data.StdDevTPS), fmt.Sprintf("%.2f", data.MaxTPS), "个/秒", "输出 tokens / 总耗时")
	table.Append("🌐 吞吐 TPS", fmt.Sprintf("%.2f", data.MinTotalThroughputTPS), fmt.Sprintf("%.2f", data.AvgTotalThroughputTPS), fmt.Sprintf("±%.2f", data.StdDevTotalThroughputTPS), fmt.Sprintf("%.2f", data.MaxTotalThroughputTPS), "个/秒", "(输入+输出) tokens / 总耗时")

	// Token 数指标
	table.Append("📥 输入 Token 数", strconv.Itoa(data.MinInputTokenCount), strconv.Itoa(data.AvgInputTokenCount), fmt.Sprintf("±%.2f", data.StdDevInputTokenCount), strconv.Itoa(data.MaxInputTokenCount), "个", "API 请求的 prompt tokens")
	table.Append("🎲 生成 Token 数", strconv.Itoa(data.MinOutputTokenCount), strconv.Itoa(data.AvgOutputTokenCount), fmt.Sprintf("±%.2f", data.StdDevOutputTokenCount), strconv.Itoa(data.MaxOutputTokenCount), "个", "API 返回的 completion tokens")
	table.Append("🧠 思考 Token 数", strconv.Itoa(data.MinThinkingTokenCount), strconv.Itoa(data.AvgThinkingTokenCount), fmt.Sprintf("±%.2f", data.StdDevThinkingTokenCount), strconv.Itoa(data.MaxThinkingTokenCount), "个", "模型返回的 reasoning/thinking tokens")

	table.Render()
	fmt.Println()
}

// 将数据更新到终端上（刷新显示）
// 概览模式，每行一个，展示主要数据（平均值）
func (td *Displayer) ShowMultiReport(data []*types.ReportData) {
	// 单个汇总表格，包含所有不同类型指标的平均值
	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithEastAsian(false),
	)

	table.Header("🤖 模型", "📊 请求数", "⚡ 并发", "✅ 成功率",
		"🕐 平均总耗时", "⚡ 平均 TTFT",
		"🚀 平均输出 TPS", "🌐 平均吞吐 TPS",
		"🎲 平均输出 Token 数")

	for _, report := range data {
		// TTFT 处理（流式模式才显示）
		ttftStr := "-"
		if report.IsStream {
			ttftStr = formatDuration(report.AvgTTFT)
		}

		table.Append(
			report.Model,
			strconv.Itoa(report.TotalRequests),
			strconv.Itoa(report.Concurrency),
			fmt.Sprintf("%.2f%%", report.SuccessRate),
			formatDuration(report.AvgTotalTime),
			ttftStr,
			fmt.Sprintf("%.2f", report.AvgTPS),
			fmt.Sprintf("%.2f", report.AvgTotalThroughputTPS),
			strconv.Itoa(report.AvgOutputTokenCount),
		)
	}

	table.Render()
	fmt.Println()
}

// maskApiKey 隐藏 API 密钥的敏感部分
func maskApiKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
}

// truncatePrompt 截断过长的提示词并显示长度信息
func truncatePrompt(prompt string) string {
	runes := []rune(prompt)
	charCount := len(runes)
	if charCount <= 50 {
		return fmt.Sprintf("%s (长度: %d)", prompt, charCount)
	}
	return fmt.Sprintf("%s... (长度: %d)", string(runes[:47]), charCount)
}

// formatDuration 格式化时间显示，保留2位小数
func formatDuration(d time.Duration) string {
	// 根据时间大小选择合适的单位
	if d >= time.Second {
		// >= 1s: 显示为秒，保留2位小数
		return fmt.Sprintf("%.2fs", d.Seconds())
	} else if d >= time.Millisecond {
		// >= 1ms: 显示为毫秒，保留2位小数
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	} else if d >= time.Microsecond {
		// >= 1µs: 显示为微秒，保留2位小数
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	}
	// < 1µs: 显示为纳秒
	return fmt.Sprintf("%dns", d.Nanoseconds())
}
