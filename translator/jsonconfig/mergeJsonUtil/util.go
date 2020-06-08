package mergeJsonUtil

import (
	"fmt"
	"reflect"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
)

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func MergeMap(source map[string]interface{}, result map[string]interface{}, sectionKey string,
	mergeRuleMap map[string]mergeJsonRule.MergeRule, path string) {
	subMapSource, exists := GetSubMap(source, sectionKey)
	subMapResult, _ := GetSubMap(result, sectionKey)
	if !exists {
		return
	}
	if len(subMapSource) == 0 {
		result[sectionKey] = subMapResult
		return
	}
	if len(subMapResult) == 0 {
		result[sectionKey] = subMapResult
	}
	mergeMap(subMapSource, subMapResult, mergeRuleMap, path)
}

func mergeMap(sourceMap map[string]interface{}, resultMap map[string]interface{}, mergeRuleMap map[string]mergeJsonRule.MergeRule, path string) {
	for key, value := range sourceMap {
		if rule, ok := mergeRuleMap[key]; ok {
			rule.Merge(sourceMap, resultMap)
		} else if existingValue, ok := resultMap[key]; !ok {
			// only one defines the value
			resultMap[key] = value
		} else if !reflect.DeepEqual(existingValue, value) {
			// fail if different values are defined
			translator.AddErrorMessages(fmt.Sprintf("%s%s", path, key), fmt.Sprintf("Different values are specified for %v", key))
		}
		// the same value is defined by multiple sources
	}
}

func MergeList(source map[string]interface{}, result map[string]interface{}, sectionKey string) {
	subListSource := GetSubList(source, sectionKey)
	if len(subListSource) == 0 {
		return
	}
	subListResult := GetSubList(result, sectionKey)
	if len(subListResult) == 0 {
		result[sectionKey] = subListResult
	}
	subListResult = mergeList(subListSource, subListResult)
	result[sectionKey] = subListResult
}

func mergeList(sourceList []interface{}, destList []interface{}) []interface{} {
	for _, value := range sourceList {
		shouldMerge := true
	Loop:
		for _, existingValue := range destList {
			if reflect.DeepEqual(existingValue, value) {
				// the same value is defined by multiple sources
				shouldMerge = false
				break Loop
			}
		}

		// the value is not defined yet, since it is a list, merge the different value
		if shouldMerge {
			destList = append(destList, value)
		}
	}
	return destList
}

func GetSubMap(sourceMap map[string]interface{}, subKey string) (map[string]interface{}, bool) {
	resultMap := map[string]interface{}{}
	var subObj interface{}
	var ok bool
	if subObj, ok = sourceMap[subKey]; !ok {
		return resultMap, false
	}
	if subMap, ok := subObj.(map[string]interface{}); ok {
		return subMap, true
	}
	return resultMap, false
}

func GetSubList(sourceMap map[string]interface{}, subKey string) []interface{} {
	resultList := make([]interface{}, 0)
	var subObj interface{}
	var ok bool
	if subObj, ok = sourceMap[subKey]; !ok {
		return resultList
	}
	if subList, ok := subObj.([]interface{}); ok {
		return subList
	}
	return resultList
}
