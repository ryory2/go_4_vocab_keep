package webutil

import (
	"log"
	"reflect"
	"strings"

	"github.com/go-playground/locales/ja" // 日本語ロケール
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	ja_translations "github.com/go-playground/validator/v10/translations/ja" // 日本語翻訳
)

// Validator はアプリケーション全体で共有されるバリデータインスタンスです。
var Validator *validator.Validate

// Trans はエラーメッセージを翻訳するためのトランスレータです。
var Trans ut.Translator

var fieldNameTranslations = map[string]string{
	"name":       "名前",
	"term":       "単語",
	"definition": "意味",
	"email":      "メールアドレス",
	"is_correct": "回答の正誤",
	// ... 他のフィールドもここに追加 ...
}

func init() {
	// バリデータのインスタンスを生成
	Validator = validator.New()

	// JSONタグからフィールド名を取得するように設定
	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// --- ここからが日本語化の処理 ---

	// 日本語のロケールとトランスレータを設定
	japanese := ja.New()
	uni := ut.New(japanese, japanese)
	var found bool
	Trans, found = uni.GetTranslator("ja")
	if !found {
		log.Fatal("translator not found")
	}

	// バリデータに日本語の翻訳を登録
	if err := ja_translations.RegisterDefaultTranslations(Validator, Trans); err != nil {
		log.Fatal(err)
	}

	// 必要に応じて、個別のエラーメッセージを上書き・カスタマイズ
	// registerTranslation は、メッセージテンプレートを登録するヘルパー関数
	registerTranslation := func(tag string, msg string) {
		Validator.RegisterTranslation(tag, Trans, func(ut ut.Translator) error {
			return ut.Add(tag, msg, true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			// ★ここからが修正部分

			// 1. jsonタグ名を取得 (e.g., "term")
			fieldName := fe.Field()

			// 2. マップから日本語名を取得
			translatedFieldName, ok := fieldNameTranslations[fieldName]
			if !ok {
				// マップにない場合は、元のjsonタグ名をそのまま使う
				translatedFieldName = fieldName
			}

			// 3. 翻訳されたフィールド名を使ってメッセージを生成
			t, _ := ut.T(tag, translatedFieldName)
			return t
		})
	}

	// 例: "required" タグのメッセージをよりシンプルにする
	registerTranslation("required", "{0}は必須項目です。")
	// 例: "email" タグのメッセージ
	registerTranslation("email", "{0}は有効なメールアドレス形式ではありません。")
	// --- min タグの修正 ---
	Validator.RegisterTranslation("min", Trans, func(ut ut.Translator) error {
		// メッセージテンプレートの登録
		return ut.Add("min", "{0}は{1}文字以上で入力してください。", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		// メッセージ生成ロジック
		fieldName := fe.Field()
		translatedFieldName, ok := fieldNameTranslations[fieldName]
		if !ok {
			translatedFieldName = fieldName // 見つからなければ元の名前
		}
		// 翻訳されたフィールド名を使ってメッセージを生成
		t, _ := ut.T("min", translatedFieldName, fe.Param())
		return t
	})

	// --- max タグの修正 ---
	Validator.RegisterTranslation("max", Trans, func(ut ut.Translator) error {
		// メッセージテンプレートの登録
		return ut.Add("max", "{0}は{1}文字以下で入力してください。", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		// メッセージ生成ロジック
		fieldName := fe.Field()
		translatedFieldName, ok := fieldNameTranslations[fieldName]
		if !ok {
			translatedFieldName = fieldName // 見つからなければ元の名前
		}
		// 翻訳されたフィールド名を使ってメッセージを生成
		t, _ := ut.T("max", translatedFieldName, fe.Param())
		return t
	})
}
