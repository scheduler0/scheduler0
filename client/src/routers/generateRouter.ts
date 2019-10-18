// @ts-ignore:
import express from "express";
import axiosInstance from "../misc/axiosInstance";

export const generateRouter = (pathName: string) => {
    const router = express.Router();

    router.route("/")
        .post((req, res) => {
            axiosInstance
                .post(`/${pathName}`, req.body)
                .then((resp) => {
                    res.status(resp.status).send(resp.data);
                }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
        })
        .get(async (_, res) => {
            let resp;

            try {
                resp = await axiosInstance.get(`/${pathName}`);
                res.status(resp.status).send(resp.data);
            } catch (e) {
                res.status(e.status).send(e.data);
            }
        });

    router.route("/:id")
        .delete(async (req, res) => {
            axiosInstance.delete(`/${pathName}/${req.params.id}`)
                .then((resp) => {
                    res.status(resp.status).send(resp.data);
                }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
        })
        .get(async (req, res) => {
            await axiosInstance.get(`/${pathName}/${req.params.id}`)
                .then((resp) => {
                    res.status(resp.status).send(resp.data);
                }).catch((e) => {
                    res.status(e.response.status).send(e.response.data);
                });
        })
        .put(async (req, res) => {
            await axiosInstance.put(`/${pathName}/${req.params.id}`, req.body)
                .then((resp) => {
                    res.status(resp.status).send(resp.data);
                }).catch((e) => {
                    res.status(e.response.status).send(e.response.data);
                });
        });

    return router;
};