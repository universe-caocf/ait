package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/yinxulai/ait/internal/prompt"
)

func defaultOutputPath(length int) string {
	return filepath.Join("ait_automation", fmt.Sprintf("generated_prompt_%d.txt", length))
}

func resolveOutputPath(length int, output string) string {
	if output != "" {
		return output
	}
	return defaultOutputPath(length)
}

func validateLength(length int) error {
	if length <= 0 {
		return fmt.Errorf("length 必须大于 0")
	}
	return nil
}

func generatePrompt(length int) (string, error) {
	if err := validateLength(length); err != nil {
		return "", err
	}
	return prompt.GeneratePromptByLength(length), nil
}

func writePromptFile(outputPath, content string) error {
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入 prompt 文件失败: %w", err)
	}

	return nil
}

func run(length int, output string) error {
	content, err := generatePrompt(length)
	if err != nil {
		return err
	}

	outputPath := resolveOutputPath(length, output)
	if err := writePromptFile(outputPath, content); err != nil {
		return err
	}

	actualLength := utf8.RuneCountInString(content)
	fmt.Printf("prompt 已生成: %s (length=%d)\n", outputPath, actualLength)
	return nil
}

func main() {
	length := flag.Int("length", 0, "目标 prompt 字符数（必填，按 rune 计数）")
	output := flag.String("output", "", "输出 txt 文件路径（可选）")
	flag.Parse()

	if err := run(*length, *output); err != nil {
		fmt.Fprintf(os.Stderr, "生成 prompt 失败: %v\n", err)
		os.Exit(1)
	}
}
