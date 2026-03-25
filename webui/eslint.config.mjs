import pluginVue from "eslint-plugin-vue";

export default [
    {
        ignores: ["**/dist/**", "**/public/**", "**/.yarn/**", "**/node_modules/**"],
        },
    ...pluginVue.configs["flat/essential"],
    {
        rules: {
        "vue/multi-word-component-names": "off",
        }
    }
];
